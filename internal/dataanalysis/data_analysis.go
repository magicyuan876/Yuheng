package dataanalysis

import (
	"context"
	"database/sql"
	"fmt"
	filesvc "github.com/magicyuan876/yuheng/internal/application/service/file"
	"io"
	"os"
	"strings"

	"github.com/magicyuan876/yuheng/internal/logger"
	"github.com/magicyuan876/yuheng/internal/types"
	"github.com/magicyuan876/yuheng/internal/types/interfaces"
	"github.com/magicyuan876/yuheng/internal/utils"
)

// excelSheetNameColumn is the name of the synthetic column that identifies
// which Excel sheet a row came from when multiple sheets are unioned together.
const excelSheetNameColumn = "__sheet_name"

// sqlSingleQuoteEscape escapes single quotes in a string so it can be safely
// embedded inside a single-quoted SQL literal.
func sqlSingleQuoteEscape(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

type DataAnalysisTool struct {
	knowledgeBaseService interfaces.KnowledgeBaseService
	knowledgeService     interfaces.KnowledgeService
	fileService          interfaces.FileService
	tenantService        interfaces.TenantService
	db                   *sql.DB
	sessionID            string
	createdTables        []string // Track tables created in this session
	// localBaseDir is the LOCAL_STORAGE_BASE_DIR value captured at construction
	// time so resolveFileServiceForKnowledge uses the same base path that was
	// used when the local FileService was initialised by DI.  Re-reading the
	// env var at request time can produce a different (or empty) value if the
	// variable was not exported to the sub-process or was set programmatically
	// after startup, causing GetFile to look in the wrong directory (#1040).
	localBaseDir    string
	storageResolver interfaces.StorageBackendResolver
}

func NewDataAnalysisTool(
	knowledgeBaseService interfaces.KnowledgeBaseService,
	knowledgeService interfaces.KnowledgeService,
	tenantService interfaces.TenantService,
	fileService interfaces.FileService,
	db *sql.DB,
	sessionID string,
	storageResolvers ...interfaces.StorageBackendResolver,
) *DataAnalysisTool {
	tool := &DataAnalysisTool{
		knowledgeBaseService: knowledgeBaseService,
		knowledgeService:     knowledgeService,
		fileService:          fileService,
		tenantService:        tenantService,
		db:                   db,
		sessionID:            sessionID,
		// Capture LOCAL_STORAGE_BASE_DIR once at construction time so that every
		// call to resolveFileServiceForKnowledge uses the same base path.  The
		// env var is guaranteed to be set (or empty == "/data/files" fallback)
		// when the application starts and the DI container is assembled.
		localBaseDir: strings.TrimSpace(os.Getenv("LOCAL_STORAGE_BASE_DIR")),
	}
	if len(storageResolvers) > 0 {
		tool.storageResolver = storageResolvers[0]
	}
	return tool
}

// recordCreatedTable records a table name for cleanup, ensuring uniqueness
// Returns true if the table was newly recorded, false if it already existed
func (t *DataAnalysisTool) recordCreatedTable(tableName string) bool {
	for _, name := range t.createdTables {
		if name == tableName {
			return false
		}
	}
	t.createdTables = append(t.createdTables, tableName)
	return true
}

// Cleanup cleans up the session-specific schema
func (t *DataAnalysisTool) Cleanup(ctx context.Context) {
	if len(t.createdTables) == 0 {
		logger.Infof(ctx, "[Tool][DataAnalysis] No tables to clean up for session: %s", t.sessionID)
		return
	}

	logger.Infof(ctx, "[Tool][DataAnalysis] Cleaning up %d tables for session: %s", len(t.createdTables), t.sessionID)

	for _, tableName := range t.createdTables {
		dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS \"%s\"", tableName)
		if _, err := t.db.ExecContext(ctx, dropSQL); err != nil {
			logger.Errorf(ctx, "[Tool][DataAnalysis] Failed to drop table '%s': %v", tableName, err)
			// Continue to drop other tables even if one fails
			continue
		}
		logger.Infof(ctx, "[Tool][DataAnalysis] Successfully dropped table '%s'", tableName)
	}

	// Clear the list after cleanup
	t.createdTables = nil
}

// RunQuery loads the knowledge into DuckDB when it is not loaded yet and
// executes a single read-only SQL query against it. The SQL may reference the
// knowledge by its ID; occurrences are rewritten to the DuckDB table name
// before execution. It returns the result rows keyed by column name.
func (t *DataAnalysisTool) RunQuery(ctx context.Context, knowledgeID string, sqlQuery string) ([]map[string]string, error) {
	logger.Infof(ctx, "[Tool][DataAnalysis] RunQuery started for session: %s", t.sessionID)

	schema, err := t.LoadFromKnowledgeID(ctx, knowledgeID)
	if err != nil {
		logger.Errorf(ctx, "[Tool][DataAnalysis] Failed to load knowledge ID '%s': %v", knowledgeID, err)
		return nil, fmt.Errorf("failed to load knowledge ID '%s': %w", knowledgeID, err)
	}

	// Replace knowledge ID with table name
	sqlQuery = strings.ReplaceAll(sqlQuery, knowledgeID, schema.TableName)

	// Check if this is a read-only query
	normalizedSQL := strings.TrimSpace(strings.ToLower(sqlQuery))
	isReadOnly := strings.HasPrefix(normalizedSQL, "select") ||
		strings.HasPrefix(normalizedSQL, "show") ||
		strings.HasPrefix(normalizedSQL, "describe") ||
		strings.HasPrefix(normalizedSQL, "explain") ||
		strings.HasPrefix(normalizedSQL, "pragma")

	if !isReadOnly {
		// Reject modification queries
		logger.Warnf(ctx, "[Tool][DataAnalysis] Modification query rejected for session %s: %s", t.sessionID, sqlQuery)
		return nil, fmt.Errorf("modification queries are not allowed: " +
			"only read-only queries (SELECT, SHOW, DESCRIBE, EXPLAIN, PRAGMA) are supported")
	}

	// Validate SQL with comprehensive security checks
	// IMPORTANT: Must enable validateSelectStmt to block RangeFunction attacks
	_, validation := utils.ValidateSQL(sqlQuery,
		utils.WithAllowedTables(schema.TableName),
		utils.WithSingleStatement(),      // Block multiple statements
		utils.WithNoDangerousFunctions(), // Block dangerous functions
	)
	if !validation.Valid {
		logger.Warnf(ctx, "[Tool][DataAnalysis] SQL validation failed for session %s: %v", t.sessionID, validation.Errors)
		return nil, fmt.Errorf("SQL validation failed: %v", validation.Errors)
	}

	logger.Infof(ctx, "[Tool][DataAnalysis] Received SQL query for session %s: %s", t.sessionID, sqlQuery)
	// Execute single query and get results
	results, err := t.executeSingleQuery(ctx, sqlQuery)
	if err != nil {
		logger.Errorf(ctx, "[Tool][DataAnalysis] Query execution failed for session %s: %v", t.sessionID, err)
		return nil, fmt.Errorf("query execution failed: %w", err)
	}

	logger.Infof(ctx, "[Tool][DataAnalysis] Completed execution query, total %d rows for session %s", len(results), t.sessionID)
	return results, nil
}

// executeSingleQuery executes a single SQL query and returns columns and results
// Parameters:
//   - ctx: context for cancellation and timeout
//   - sqlQuery: the SQL query to execute
//   - existingColumns: existing column names to merge with (can be nil or empty)
//
// Returns:
//   - []string: merged column names (existing + new columns, deduplicated)
//   - []map[string]string: query results
//   - error: any error that occurred during execution
func (t *DataAnalysisTool) executeSingleQuery(ctx context.Context, sqlQuery string) ([]map[string]string, error) {
	rows, err := t.db.QueryContext(ctx, sqlQuery)
	if err != nil {
		logger.Errorf(ctx, "[Tool][DataAnalysis] Query execution failed: %v", err)
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		logger.Errorf(ctx, "[Tool][DataAnalysis] Failed to get columns: %v", err)
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	// Process results
	results := make([]map[string]string, 0)
	for rows.Next() {
		columnValues := make([]interface{}, len(columns))
		columnPointers := make([]interface{}, len(columns))
		for i := range columnValues {
			columnPointers[i] = &columnValues[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			logger.Errorf(ctx, "[Tool][DataAnalysis] Failed to scan row: %v", err)
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		rowMap := make(map[string]string)
		for i, colName := range columns {
			val := columnValues[i]
			// Convert []byte to string for better readability
			if b, ok := val.([]byte); ok {
				rowMap[colName] = string(b)
			} else {
				rowMap[colName] = fmt.Sprintf("%v", val)
			}
		}
		results = append(results, rowMap)
	}

	if err := rows.Err(); err != nil {
		logger.Errorf(ctx, "[Tool][DataAnalysis] Error iterating rows: %v", err)
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

// TableSchema represents the schema information of a table
type TableSchema struct {
	TableName string                 `json:"table_name"`
	Columns   []ColumnInfo           `json:"columns"`
	RowCount  int64                  `json:"row_count"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ColumnInfo represents information about a single column
type ColumnInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable string `json:"nullable"`
}

// LoadFromCSV loads data from a CSV file into a DuckDB table and returns the table schema
// Parameters:
//   - ctx: context for cancellation and timeout
//   - filename: path to the CSV file
//   - tableName: name of the table to create
//
// Returns:
//   - *TableSchema: schema information of the created table
//   - error: any error that occurred during the operation
func (t *DataAnalysisTool) LoadFromCSV(ctx context.Context, filename string, tableName string) (*TableSchema, error) {
	logger.Infof(ctx, "[Tool][DataAnalysis] Loading CSV file '%s' into table '%s' for session %s", filename, tableName, t.sessionID)

	// Record the created table for cleanup. If already exists, skip creation
	if t.recordCreatedTable(tableName) {
		// Create table from CSV using DuckDB's read_csv_auto function
		// with explicit header detection and VARCHAR coercion to align with
		// Excel loading behavior.
		// Table will be created in the session schema
		createTableSQL := fmt.Sprintf(
			"CREATE TABLE \"%s\" AS SELECT * FROM read_csv_auto('%s', header=true, all_varchar=true)",
			tableName, sqlSingleQuoteEscape(filename),
		)

		_, err := t.db.ExecContext(ctx, createTableSQL)
		if err != nil {
			logger.Errorf(ctx, "[Tool][DataAnalysis] Failed to create table from CSV: %v", err)
			return nil, fmt.Errorf("failed to create table from CSV: %w", err)
		}

		logger.Infof(ctx, "[Tool][DataAnalysis] Successfully created table '%s' from CSV file in session %s", tableName, t.sessionID)
	}

	// Get and return the table schema
	return t.LoadFromTable(ctx, tableName)
}

// LoadFromExcel loads data from an Excel file into a DuckDB table and returns the table schema.
//
// Multi-sheet workbooks are fully supported: every sheet in the workbook is
// loaded and the rows from all sheets are unioned (UNION ALL BY NAME) into a
// single table. A synthetic '__sheet_name' column is added so downstream SQL
// can filter / aggregate per sheet. If sheet enumeration fails for any
// reason, we fall back to reading just the first sheet (original behavior).
//
// Parameters:
//   - ctx: context for cancellation and timeout
//   - filename: path to the Excel file
//   - tableName: name of the table to create
//
// Returns:
//   - *TableSchema: schema information of the created table
//   - error: any error that occurred during the operation
//
// Note: requires the DuckDB 'excel' extension (for read_xlsx) and the
// 'spatial' extension (for st_read_meta used to enumerate sheets).
func (t *DataAnalysisTool) LoadFromExcel(ctx context.Context, filename string, tableName string) (*TableSchema, error) {
	logger.Infof(ctx, "[Tool][DataAnalysis] Loading Excel file '%s' into table '%s' for session %s", filename, tableName, t.sessionID)

	// Record the created table for cleanup. If already exists, skip creation.
	if t.recordCreatedTable(tableName) {
		sheetNames, enumErr := t.listExcelSheets(ctx, filename)
		if enumErr != nil {
			logger.Warnf(ctx,
				"[Tool][DataAnalysis] Could not enumerate sheets for '%s' (session=%s): %v. Falling back to first sheet only.",
				filename, t.sessionID, enumErr,
			)
		}

		createTableSQL := buildExcelCreateTableSQL(tableName, filename, sheetNames)

		if _, err := t.db.ExecContext(ctx, createTableSQL); err != nil {
			logger.Errorf(ctx, "[Tool][DataAnalysis] Failed to create table from Excel (sheets=%v): %v", sheetNames, err)
			return nil, fmt.Errorf("failed to create table from Excel file (sheets=%v): %w", sheetNames, err)
		}

		logger.Infof(ctx,
			"[Tool][DataAnalysis] Successfully created table '%s' from Excel file in session %s (sheets=%v)",
			tableName, t.sessionID, sheetNames,
		)
	}

	// Get and return the table schema
	return t.LoadFromTable(ctx, tableName)
}

// listExcelSheets returns the names of every sheet (layer) inside the given
// Excel workbook by querying DuckDB's spatial st_read_meta table function.
// The returned slice preserves the on-disk order of sheets.
//
// st_read_meta returns a single row whose `layers` column is a LIST of
// STRUCTs (one per layer / sheet). We UNNEST that list and project the
// struct's `name` field to get a flat list of sheet names.
func (t *DataAnalysisTool) listExcelSheets(ctx context.Context, filename string) ([]string, error) {
	metaSQL := fmt.Sprintf(
		"SELECT UNNEST(layers).name FROM st_read_meta('%s')",
		sqlSingleQuoteEscape(filename),
	)

	rows, err := t.db.QueryContext(ctx, metaSQL)
	if err != nil {
		return nil, fmt.Errorf("failed to query sheet metadata: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan sheet name: %w", err)
		}
		if strings.TrimSpace(name) == "" {
			continue
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sheet metadata rows: %w", err)
	}
	return names, nil
}

// buildExcelCreateTableSQL assembles the CREATE TABLE statement used by
// LoadFromExcel. Exposed at package level (lower-case) to make it trivially
// testable without a live DuckDB connection.
func buildExcelCreateTableSQL(tableName, filename string, sheetNames []string) string {
	escFile := sqlSingleQuoteEscape(filename)

	// No sheet info (enumeration failed or empty): read the first sheet only.
	if len(sheetNames) == 0 {
		return fmt.Sprintf(
			"CREATE TABLE \"%s\" AS SELECT * FROM read_xlsx('%s', header=true, all_varchar=true)",
			tableName, escFile,
		)
	}

	// Single sheet: keep it simple but still tag the source for consistency
	// with the multi-sheet path.
	if len(sheetNames) == 1 {
		escSheet := sqlSingleQuoteEscape(sheetNames[0])
		return fmt.Sprintf(
			"CREATE TABLE \"%s\" AS SELECT *, '%s' AS %s FROM read_xlsx('%s', sheet = '%s', header=true, all_varchar=true)",
			tableName, escSheet, excelSheetNameColumn, escFile, escSheet,
		)
	}

	// Multiple sheets: UNION ALL BY NAME tolerates schema differences
	// between sheets (missing columns become NULL, conflicting types are
	// widened).
	parts := make([]string, 0, len(sheetNames))
	for _, sheet := range sheetNames {
		escSheet := sqlSingleQuoteEscape(sheet)
		parts = append(parts, fmt.Sprintf(
			"SELECT *, '%s' AS %s FROM read_xlsx('%s', sheet = '%s', header=true, all_varchar=true)",
			escSheet, excelSheetNameColumn, escFile, escSheet,
		))
	}
	return fmt.Sprintf(
		"CREATE TABLE \"%s\" AS %s",
		tableName,
		strings.Join(parts, "\nUNION ALL BY NAME\n"),
	)
}

// LoadFromKnowledge loads data from a Knowledge entity into a DuckDB table and returns the table schema.
// It automatically determines the file type and calls the appropriate loading method.
//
// The source file is first materialized to a local temp file via FileService.GetFile
// so DuckDB's st_read / read_xlsx / read_csv_auto can open it directly. This
// side-steps provider-specific URL schemes (e.g. the local:// URL returned by
// the local file service) that DuckDB's extensions cannot resolve on their own.
//
// Parameters:
//   - ctx: context for cancellation and timeout
//   - knowledge: the Knowledge entity containing file information
//
// Returns:
//   - *TableSchema: schema information of the created table
//   - error: any error that occurred during the operation
func (t *DataAnalysisTool) LoadFromKnowledge(ctx context.Context, knowledge *types.Knowledge) (*TableSchema, error) {
	if knowledge == nil {
		return nil, fmt.Errorf("knowledge cannot be nil")
	}
	tableName := t.TableName(knowledge)

	// Normalize file type to lowercase for comparison
	fileType := strings.ToLower(knowledge.FileType)

	logger.Infof(ctx, "[Tool][DataAnalysis] Loading knowledge '%s' (type: %s) into table '%s' for session %s",
		knowledge.ID, fileType, tableName, t.sessionID)

	localPath, cleanup, err := t.materializeKnowledgeFile(ctx, knowledge)
	if err != nil {
		return nil, fmt.Errorf("failed to materialize knowledge '%s' for DuckDB: %w", knowledge.ID, err)
	}
	defer cleanup()

	switch fileType {
	case "csv":
		return t.LoadFromCSV(ctx, localPath, tableName)
	case "xlsx", "xls":
		return t.LoadFromExcel(ctx, localPath, tableName)
	default:
		logger.Warnf(ctx, "[Tool][DataAnalysis] Unsupported file type '%s' for knowledge '%s' in session %s",
			fileType, knowledge.ID, t.sessionID)
		return nil, fmt.Errorf("unsupported file type: %s (supported types: csv, xlsx, xls)", fileType)
	}
}

// materializeKnowledgeFile copies the knowledge's backing blob into a fresh
// temp file on the local filesystem so DuckDB can open it with ordinary path
// semantics. It returns the temp path and a cleanup closure that removes the
// temp file; the closure is always safe to call and is a no-op on failure.
//
// This hides storage-backend-specific URL schemes (local://, s3://, …) behind
// the FileService.GetFile abstraction, so the Data Analysis tool works
// identically across all deployments.
func (t *DataAnalysisTool) materializeKnowledgeFile(ctx context.Context, knowledge *types.Knowledge) (string, func(), error) {
	noop := func() {}

	reader, err := t.resolveFileServiceForKnowledge(ctx, knowledge).GetFile(ctx, knowledge.FilePath)
	if err != nil {
		return "", noop, fmt.Errorf("failed to open file for knowledge '%s': %w", knowledge.ID, err)
	}
	defer reader.Close()

	// Preserve the file extension so DuckDB's format auto-detection still
	// works (e.g. the CSV reader expects .csv, xlsx reader expects .xlsx).
	suffix := ""
	if ext := strings.ToLower(strings.TrimSpace(knowledge.FileType)); ext != "" {
		suffix = "." + ext
	}

	tmp, err := os.CreateTemp("", "yuheng-data-analysis-*"+suffix)
	if err != nil {
		return "", noop, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		// Best-effort cleanup; a missing file is fine, any other error is
		// only logged to avoid masking the original operation's result.
		if err := os.Remove(tmpPath); err != nil && !os.IsNotExist(err) {
			logger.Warnf(ctx, "[Tool][DataAnalysis] Failed to remove temp file %s: %v", tmpPath, err)
		}
	}

	if _, err := io.Copy(tmp, reader); err != nil {
		_ = tmp.Close()
		cleanup()
		return "", noop, fmt.Errorf("failed to copy knowledge '%s' to temp file: %w", knowledge.ID, err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", noop, fmt.Errorf("failed to finalize temp file for knowledge '%s': %w", knowledge.ID, err)
	}

	logger.Infof(ctx, "[Tool][DataAnalysis] Materialized knowledge '%s' to temp file %s for session %s",
		knowledge.ID, tmpPath, t.sessionID)

	return tmpPath, cleanup, nil
}

// LoadFromKnowledgeID loads data from a Knowledge ID into a DuckDB table and returns the table schema
// Parameters:
//   - ctx: context for cancellation and timeout
//   - knowledgeID: the ID of the Knowledge entity
//
// Returns:
//   - string: the name of the created table
//   - *TableSchema: schema information of the created table
//   - error: any error that occurred during the operation
func (t *DataAnalysisTool) LoadFromKnowledgeID(ctx context.Context, knowledgeID string) (*TableSchema, error) {
	// Use GetKnowledgeByIDOnly to support cross-tenant shared KB
	knowledge, err := t.knowledgeService.GetKnowledgeByIDOnly(ctx, knowledgeID)
	if err != nil || knowledge == nil {
		if err == nil {
			err = fmt.Errorf("knowledge service returned an empty result")
		}
		logger.Errorf(ctx, "[Tool][DataAnalysis] Failed to get knowledge by ID '%s': %v", knowledgeID, err)
		return nil, fmt.Errorf("failed to get knowledge by ID: %w", err)
	}

	return t.LoadFromKnowledge(ctx, knowledge)
}

// LoadFromTable retrieves the schema information of an existing table
// Parameters:
//   - ctx: context for cancellation and timeout
//   - tableName: name of the table to query
//
// Returns:
//   - *TableSchema: schema information of the table
//   - error: any error that occurred during the operation
//
// Note: This function does NOT create the table, it only retrieves schema information
func (t *DataAnalysisTool) LoadFromTable(ctx context.Context, tableName string) (*TableSchema, error) {
	logger.Infof(ctx, "[Tool][DataAnalysis] Getting schema for table '%s' in session %s", tableName, t.sessionID)

	// Query to get column information using PRAGMA table_info or DESCRIBE
	schemaSQL := fmt.Sprintf("DESCRIBE \"%s\"", tableName)

	rows, err := t.db.QueryContext(ctx, schemaSQL)
	if err != nil {
		logger.Errorf(ctx, "[Tool][DataAnalysis] Failed to get table schema: %v", err)
		return nil, fmt.Errorf("failed to get table schema: %w", err)
	}
	defer rows.Close()

	// Parse column information
	columns := make([]ColumnInfo, 0)
	for rows.Next() {
		var colName, colType, nullable string
		var extra1, extra2, extra3 interface{} // DuckDB DESCRIBE may return additional columns

		// Try to scan with different column counts
		err := rows.Scan(&colName, &colType, &nullable, &extra1, &extra2, &extra3)
		if err != nil {
			// Try with fewer columns
			err = rows.Scan(&colName, &colType, &nullable)
			if err != nil {
				logger.Errorf(ctx, "[Tool][DataAnalysis] Failed to scan column info: %v", err)
				return nil, fmt.Errorf("failed to scan column info: %w", err)
			}
		}

		columns = append(columns, ColumnInfo{
			Name:     colName,
			Type:     colType,
			Nullable: nullable,
		})
	}

	if err := rows.Err(); err != nil {
		logger.Errorf(ctx, "[Tool][DataAnalysis] Error iterating schema rows: %v", err)
		return nil, fmt.Errorf("error iterating schema rows: %w", err)
	}

	// Get row count
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM \"%s\"", tableName)
	var rowCount int64
	if err := t.db.QueryRowContext(ctx, countSQL).Scan(&rowCount); err != nil {
		logger.Errorf(ctx, "[Tool][DataAnalysis] Failed to get row count: %v", err)
		return nil, fmt.Errorf("failed to get row count: %w", err)
	}

	schema := &TableSchema{
		TableName: tableName,
		Columns:   columns,
		RowCount:  rowCount,
		Metadata: map[string]interface{}{
			"column_count": len(columns),
			"session_id":   t.sessionID,
		},
	}

	logger.Infof(ctx, "[Tool][DataAnalysis] Retrieved schema for table '%s' in session %s: %d columns, %d rows",
		tableName, t.sessionID, len(columns), rowCount)

	return schema, nil
}

func (t *DataAnalysisTool) TableName(knowledge *types.Knowledge) string {
	return "k_" + strings.ReplaceAll(knowledge.ID, "-", "_")
}

// buildSchemaDescription builds a formatted schema description
func (t *TableSchema) Description() string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Table name: %s\n", t.TableName))
	builder.WriteString(fmt.Sprintf("Columns: %d\n", len(t.Columns)))
	builder.WriteString(fmt.Sprintf("Rows: %d\n\n", t.RowCount))
	builder.WriteString("Column info:\n")

	for _, col := range t.Columns {
		builder.WriteString(fmt.Sprintf("- %s (%s)\n", col.Name, col.Type))
	}

	return builder.String()
}

// resolveFileServiceForKnowledge resolves a provider-specific FileService based on the knowledge file path.
// It falls back to the injected default service when provider/config cannot be resolved.
func (t *DataAnalysisTool) resolveFileServiceForKnowledge(ctx context.Context, knowledge *types.Knowledge) interfaces.FileService {
	if knowledge == nil {
		logger.Warnf(ctx, "[Tool][DataAnalysis][storage] fallback default: session_id=%s reason=knowledge_nil", t.sessionID)
		return t.fileService
	}

	kbID := strings.TrimSpace(knowledge.KnowledgeBaseID)
	var kb *types.KnowledgeBase
	if t.knowledgeBaseService != nil && kbID != "" {
		var err error
		kb, err = t.knowledgeBaseService.GetKnowledgeBaseByID(ctx, kbID)
		if err != nil {
			logger.Warnf(ctx, "[Tool][DataAnalysis][storage] get kb failed, fallback default: session_id=%s knowledge_id=%s kb_id=%s err=%v",
				t.sessionID, knowledge.ID, kbID, err)
			return t.fileService
		}
	}
	if kb == nil && kbID != "" {
		logger.Infof(ctx, "[Tool][DataAnalysis][storage] kb not found, fallback default: session_id=%s knowledge_id=%s kb_id=%s",
			t.sessionID, knowledge.ID, kbID)
		return t.fileService
	}

	provider := ""
	backendID, _, _ := types.ParseStorageBackendPath(knowledge.FilePath)
	if kb != nil {
		provider = kb.GetStorageProvider()
		if backendID == "" && kb.StorageBackendID != nil {
			backendID = strings.TrimSpace(*kb.StorageBackendID)
		}
	}
	tenant, _ := ctx.Value(types.TenantInfoContextKey).(*types.Tenant)
	if tenant == nil {
		tenantID := uint64(0)
		if tid, ok := ctx.Value(types.TenantIDContextKey).(uint64); ok {
			tenantID = tid
		}
		if tenantID == 0 && kb != nil {
			tenantID = knowledge.TenantID
		}
		if tenantID > 0 && t.tenantService != nil {
			resolvedTenant, err := t.tenantService.GetTenantByID(ctx, tenantID)
			if err != nil {
				logger.Warnf(ctx, "[Tool][DataAnalysis][storage] get tenant failed: session_id=%s knowledge_id=%s kb_id=%s tenant_id=%d err=%v",
					t.sessionID, knowledge.ID, kbID, tenantID, err)
			} else if resolvedTenant != nil {
				tenant = resolvedTenant
				logger.Infof(ctx, "[Tool][DataAnalysis][storage] resolved tenant from service: session_id=%s knowledge_id=%s kb_id=%s tenant_id=%d",
					t.sessionID, knowledge.ID, kbID, tenantID)
			}
		}
	}
	if provider == "" && tenant != nil && tenant.StorageEngineConfig != nil {
		provider = strings.ToLower(strings.TrimSpace(tenant.StorageEngineConfig.DefaultProvider))
	}
	if t.storageResolver != nil && tenant != nil && (backendID != "" || provider != "") {
		resolvedSvc, resolvedProvider, err := t.storageResolver.ResolveFileService(
			ctx, tenant, backendID, provider, t.localBaseDir,
		)
		if err == nil {
			logger.Infof(ctx, "[Tool][DataAnalysis][storage] resolved storage backend: session_id=%s knowledge_id=%s kb_id=%s backend_id=%s provider=%s",
				t.sessionID, knowledge.ID, kbID, backendID, resolvedProvider)
			return resolvedSvc
		}
		logger.Warnf(ctx, "[Tool][DataAnalysis][storage] resolve storage backend failed, trying legacy config: session_id=%s knowledge_id=%s kb_id=%s backend_id=%s provider=%s err=%v",
			t.sessionID, knowledge.ID, kbID, backendID, provider, err)
	}

	if provider == "" || tenant == nil || tenant.StorageEngineConfig == nil {
		hasTenantStorageConfig := tenant != nil && tenant.StorageEngineConfig != nil
		logger.Infof(ctx, "[Tool][DataAnalysis][storage] fallback default: session_id=%s knowledge_id=%s kb_id=%s provider=%q tenant_cfg=%t",
			t.sessionID, knowledge.ID, kbID, provider, hasTenantStorageConfig)
		return t.fileService
	}

	storageConfig := tenant.StorageEngineConfig
	// Use the localBaseDir captured at construction time rather than re-reading
	// LOCAL_STORAGE_BASE_DIR from os.Getenv here.  Reading the env var at
	// request-handling time can produce an empty string (or the wrong value)
	// when the variable was set programmatically before startup or is absent
	// from the process environment of the DI-constructed sub-component, causing
	// the newly created local FileService to use the /data/files fallback
	// instead of the configured path and therefore fail to locate files (#1040).
	baseDir := t.localBaseDir

	resolvedSvc, resolvedProvider, err := filesvc.NewFileServiceFromStorageConfig(provider, storageConfig, baseDir)
	if err != nil {
		logger.Warnf(ctx, "[Tool][DataAnalysis][storage] create file service failed, fallback default: session_id=%s knowledge_id=%s kb_id=%s provider=%s err=%v",
			t.sessionID, knowledge.ID, kbID, provider, err)
		return t.fileService
	}

	logger.Infof(ctx, "[Tool][DataAnalysis][storage] resolved file service: session_id=%s knowledge_id=%s kb_id=%s provider=%s",
		t.sessionID, knowledge.ID, kbID, resolvedProvider)
	return resolvedSvc
}
