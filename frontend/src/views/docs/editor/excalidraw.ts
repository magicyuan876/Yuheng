// Loading the Excalidraw editor, and turning what it holds into the two files
// a diagram is stored as.
//
// The module is imported dynamically and never at start-up. It is by a wide
// margin the largest thing in this application, it is needed only by somebody
// who opens a drawing for editing, and most readers of most pages never do.
// Everything here therefore has to survive the import resolving late, or
// failing, or resolving after the component that asked for it has gone.

/** The parts of the Excalidraw module this integration uses. */
export interface ExcalidrawModule {
  Excalidraw: unknown
  exportToSvg: (options: {
    elements: readonly unknown[]
    appState: Record<string, unknown>
    files: unknown
    exportPadding?: number
  }) => Promise<SVGSVGElement>
}

/** One drawing, as Excalidraw stores it. */
export interface ExcalidrawScene {
  type: 'excalidraw'
  version: number
  source: string
  elements: unknown[]
  appState: Record<string, unknown>
  files: Record<string, unknown>
}

/** The scene a new drawing starts from. */
export function emptyScene(): ExcalidrawScene {
  return {
    type: 'excalidraw',
    version: 2,
    source: 'yuheng',
    elements: [],
    appState: {},
    files: {},
  }
}

/**
 * Reads a stored scene.
 *
 * A scene that will not parse, or that is not a scene at all, opens as a blank
 * canvas rather than throwing: the alternative is an editor that cannot be
 * opened at all on a file somebody can then never repair.
 */
export function parseScene(raw: string): ExcalidrawScene {
  try {
    const parsed = JSON.parse(raw) as Partial<ExcalidrawScene> | null
    if (!parsed || typeof parsed !== 'object') return emptyScene()
    return {
      ...emptyScene(),
      elements: Array.isArray(parsed.elements) ? parsed.elements : [],
      appState: isRecord(parsed.appState) ? sanitiseAppState(parsed.appState) : {},
      files: isRecord(parsed.files) ? parsed.files : {},
    }
  } catch {
    return emptyScene()
  }
}

/** Serialises a scene for storage. */
export function serialiseScene(
  elements: readonly unknown[],
  appState: Record<string, unknown>,
  files: Record<string, unknown>,
): string {
  return JSON.stringify({
    ...emptyScene(),
    elements: [...elements],
    appState: sanitiseAppState(appState),
    files,
  })
}

/**
 * Keeps only the parts of the editor's state that describe the drawing.
 *
 * The rest is this session's view: where the canvas was scrolled, what was
 * selected, whether a dialogue was open. Storing it would make two people
 * saving the same unchanged drawing produce different files, and would reopen
 * somebody else's scroll position as if it were part of the diagram.
 */
export function sanitiseAppState(appState: Record<string, unknown>): Record<string, unknown> {
  const keep = [
    'gridSize', 'viewBackgroundColor', 'exportBackground', 'exportWithDarkMode',
    'exportEmbedScene', 'exportScale', 'frameRendering',
  ]
  const out: Record<string, unknown> = {}
  for (const key of keep) {
    if (key in appState) out[key] = appState[key]
  }
  return out
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value)
}

/**
 * Renders a scene to SVG text, which is what every reader actually sees.
 *
 * The rendering is produced by the editor itself rather than by anything here,
 * so it looks exactly like what the author was drawing. It is stored alongside
 * the scene, and it alone is what the read-only view, the export and the share
 * page load.
 */
export async function renderSceneToSVG(
  module: ExcalidrawModule,
  scene: ExcalidrawScene,
): Promise<string> {
  const svg = await module.exportToSvg({
    elements: scene.elements,
    appState: { ...scene.appState, exportBackground: false },
    files: scene.files,
    exportPadding: 8,
  })
  return new XMLSerializer().serializeToString(svg)
}

/** Caches the dynamic import so opening a second drawing is instant. */
let loading: Promise<ExcalidrawModule> | null = null

/**
 * Loads the editor.
 *
 * A failed import does not poison the cache: the promise is cleared so the
 * next attempt tries again, which matters on a flaky connection where the
 * chunk is several megabytes.
 */
export function loadExcalidraw(): Promise<ExcalidrawModule> {
  if (!loading) {
    loading = (async () => {
      const [module] = await Promise.all([
        import('@excalidraw/excalidraw'),
        import('@excalidraw/excalidraw/index.css'),
      ])
      return module as unknown as ExcalidrawModule
    })().catch((err) => {
      loading = null
      throw err
    })
  }
  return loading
}
