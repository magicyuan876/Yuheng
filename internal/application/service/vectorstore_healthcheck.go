package service

import (
	"context"
	"fmt"

	"github.com/magicyuan876/yuheng/internal/errors"
	"github.com/magicyuan876/yuheng/internal/types"
)

// TestConnection tests connectivity to a vector database through the probe its
// engine declares. It returns the detected server version on success (e.g.
// "7.10.1"), or an empty string if the server does not say.
func (s *vectorStoreService) TestConnection(
	ctx context.Context,
	engineType types.RetrieverEngineType,
	config types.ConnectionConfig,
) (string, error) {
	engine, ok := s.catalog.ByType(engineType)
	if !ok || engine.TestConnection == nil {
		return "", errors.NewBadRequestError(
			fmt.Sprintf("connection test not supported for engine type: %s", engineType))
	}
	return engine.TestConnection(ctx, config)
}
