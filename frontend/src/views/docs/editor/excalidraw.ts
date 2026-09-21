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
  Excalidraw: unknown;
  exportToSvg: (options: {
    elements: readonly unknown[];
    appState: Record<string, unknown>;
    files: unknown;
    exportPadding?: number;
  }) => Promise<SVGSVGElement>;
}

/** One drawing, as Excalidraw stores it. */
export interface ExcalidrawScene {
  type: "excalidraw";
  version: number;
  source: string;
  elements: unknown[];
  appState: Record<string, unknown>;
  files: Record<string, unknown>;
}

/**
 * The style elements are drawn in when the drawing does not say otherwise.
 *
 * Left to itself the editor draws a sketch: every stroke is laid down twice
 * with a wobble, and text is set in a handwriting face whose CJK fallback is
 * another handwriting face. That is a deliberate look, and it is the wrong one
 * next to the rest of a document — a diagram in a page should read like the
 * page. So a drawing starts from clean vector strokes and a plain sans, with
 * snapping on so boxes line up without anybody nudging them.
 *
 * Two things worth knowing about the choices:
 *
 *  - Liberation Sans ships with the editor, so Latin text renders identically
 *    everywhere, including inside the exported SVG a reader sees. CJK does
 *    not ship with it — the bundled CJK face is a handwriting one, and it is
 *    only ever used as a fallback for the handwriting Latin face — so Chinese
 *    falls through to the reader's own sans. That is the intended trade: a
 *    system sans is what this should look like, and the cost is that CJK text
 *    is not byte-identical across machines.
 *  - These are the editor's own constants, written out rather than imported.
 *    Importing them would pull the whole editor — by a wide margin the largest
 *    module here — into the first paint, which is the one thing
 *    `loadExcalidraw` exists to prevent.
 */
export const DEFAULT_ITEM_STYLE: Record<string, unknown> = {
  currentItemRoughness: 0, // ROUGHNESS.architect
  currentItemFontFamily: 9, // FONT_FAMILY["Liberation Sans"]
  currentItemArrowType: "elbow",
  objectsSnapModeEnabled: true,
};

/** The scene a new drawing starts from. */
export function emptyScene(): ExcalidrawScene {
  return {
    type: "excalidraw",
    version: 2,
    source: "yuheng",
    elements: [],
    appState: {},
    files: {},
  };
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
    const parsed = JSON.parse(raw) as Partial<ExcalidrawScene> | null;
    if (!parsed || typeof parsed !== "object") return emptyScene();
    return {
      ...emptyScene(),
      elements: Array.isArray(parsed.elements) ? parsed.elements : [],
      appState: isRecord(parsed.appState) ? sanitiseAppState(parsed.appState) : {},
      files: isRecord(parsed.files) ? parsed.files : {},
    };
  } catch {
    return emptyScene();
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
  });
}

/**
 * Keeps only the parts of the editor's state that describe the drawing.
 *
 * The rest is this session's view: where the canvas was scrolled, what was
 * selected, whether a dialogue was open. Storing it would make two people
 * saving the same unchanged drawing produce different files, and would reopen
 * somebody else's scroll position as if it were part of the diagram.
 *
 * The style the next element will be drawn in is kept, though, because it
 * belongs to the drawing rather than to the session: somebody who restyled a
 * diagram and comes back to add one more box expects that box to match the
 * ones already there, not to revert to the defaults.
 */
export function sanitiseAppState(appState: Record<string, unknown>): Record<string, unknown> {
  const keep = [
    "gridSize",
    "viewBackgroundColor",
    "exportBackground",
    "exportWithDarkMode",
    "exportEmbedScene",
    "exportScale",
    "frameRendering",
    // What the next element will look like.
    "currentItemStrokeColor",
    "currentItemBackgroundColor",
    "currentItemFillStyle",
    "currentItemStrokeWidth",
    "currentItemStrokeStyle",
    "currentItemRoughness",
    "currentItemOpacity",
    "currentItemFontFamily",
    "currentItemFontSize",
    "currentItemTextAlign",
    "currentItemStartArrowhead",
    "currentItemEndArrowhead",
    "currentItemRoundness",
    "currentItemArrowType",
    // Drawing aids, which should still be on when the diagram is reopened.
    "objectsSnapModeEnabled",
    "gridModeEnabled",
  ];
  const out: Record<string, unknown> = {};
  for (const key of keep) {
    if (key in appState) out[key] = appState[key];
  }
  return out;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === "object" && !Array.isArray(value);
}

/**
 * Renders a scene to SVG text, which is what every reader actually sees.
 *
 * The rendering is produced by the editor itself rather than by anything here,
 * so it looks exactly like what the author was drawing. It is stored alongside
 * the scene, and it alone is what the read-only view, the export and the share
 * page load.
 */
export async function renderSceneToSVG(module: ExcalidrawModule, scene: ExcalidrawScene): Promise<string> {
  const svg = await module.exportToSvg({
    elements: scene.elements,
    appState: { ...scene.appState, exportBackground: false },
    files: scene.files,
    exportPadding: 8,
  });
  return new XMLSerializer().serializeToString(svg);
}

/** Caches the dynamic import so opening a second drawing is instant. */
let loading: Promise<ExcalidrawModule> | null = null;

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
        import("@excalidraw/excalidraw"),
        import("@excalidraw/excalidraw/index.css"),
      ]);
      return module as unknown as ExcalidrawModule;
    })().catch((err) => {
      loading = null;
      throw err;
    });
  }
  return loading;
}
