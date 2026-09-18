// The conversation with an embedded draw.io editor.
//
// draw.io runs in an iframe and talks over postMessage. That makes two things
// worth getting exactly right, and both are decided here rather than in a
// component, so both are tested:
//
//  1. Which messages are ours. A window with an iframe in it receives messages
//     from anything that can reach it, so a message is only acted on when it
//     came from the frame we opened, from the origin we opened it at. Checking
//     the origin is the whole of the trust boundary; without it any page in
//     any other tab could drive the editor.
//  2. What the exchange actually is. The editor announces itself, we hand it
//     the XML, it tells us when the drawing was saved, we ask for a rendered
//     copy, and it hands one back. Written as a small state machine it can be
//     stepped through in a test; written inline in a component it could only
//     be checked by hand.

/** The messages draw.io sends us, as much of them as we use. */
export interface DrawioIncoming {
  event?: string
  data?: string
  xml?: string
  message?: { key?: string }
}

/** What the editor should do next. */
export type DrawioAction =
  /** Hand the editor the drawing to open. */
  | { kind: 'load'; xml: string }
  /** Ask for a rendered copy of what is on screen. */
  | { kind: 'export' }
  /** A rendered copy arrived; store it together with the source. */
  | { kind: 'save'; xml: string; svg: string }
  /** The person closed the editor. */
  | { kind: 'close' }
  /** Nothing to do. */
  | { kind: 'none' }

/**
 * The query string that puts draw.io into the mode this editor drives.
 *
 * `proto=json` is what makes the messages parseable rather than a bespoke
 * string format, and `embed=1` is what makes it send them at all. The rest is
 * presentation: a minimal chrome, no splash, and a save button that closes.
 */
export function drawioFrameURL(base: string, dark: boolean): string {
  const url = new URL(base)
  const params = url.searchParams
  params.set('embed', '1')
  params.set('proto', 'json')
  params.set('ui', dark ? 'dark' : 'min')
  params.set('spin', '1')
  params.set('libraries', '1')
  params.set('noSaveBtn', '0')
  params.set('saveAndExit', '1')
  params.set('noExitBtn', '0')
  url.search = params.toString()
  return url.toString()
}

/** The origin an editor at this address will send messages from. */
export function originOf(base: string): string {
  try {
    return new URL(base).origin
  } catch {
    return ''
  }
}

/**
 * Decides whether a message came from the editor we opened.
 *
 * Both halves matter. The origin check is what keeps another site from
 * driving the editor; the source check is what keeps another frame on the same
 * origin from doing so. A message that fails either is not ours, and the right
 * response is to ignore it rather than to report anything: an unrelated
 * library posting to the window is ordinary, not an attack.
 */
export function isFromEditor(
  event: { origin?: string; source?: unknown },
  expectedOrigin: string,
  frame: { contentWindow?: unknown } | null,
): boolean {
  if (!expectedOrigin || event.origin !== expectedOrigin) return false
  if (!frame?.contentWindow) return false
  return event.source === frame.contentWindow
}

/** Parses a message body; anything unparseable is simply not for us. */
export function parseMessage(raw: unknown): DrawioIncoming | null {
  if (typeof raw !== 'string' || raw === '') return null
  try {
    const parsed = JSON.parse(raw) as unknown
    if (!parsed || typeof parsed !== 'object') return null
    return parsed as DrawioIncoming
  } catch {
    return null
  }
}

/** One editing session, as a state machine over the messages received. */
export class DrawioSession {
  /** The XML last handed to the editor, kept so a save can store it. */
  private xml: string
  /** Set once the editor has said it is ready. */
  private ready = false
  /** The source the editor reported at save time, pending its rendering. */
  private pendingXML: string | null = null

  constructor(xml: string) {
    this.xml = xml
  }

  /** True once the editor has announced itself. */
  get initialised(): boolean {
    return this.ready
  }

  /**
   * Steps the session on one message and says what to do.
   *
   * The caller has already established that the message is ours.
   */
  receive(message: DrawioIncoming): DrawioAction {
    switch (message.event) {
      case 'init':
        this.ready = true
        return { kind: 'load', xml: this.xml }

      case 'save':
        // The editor reports the drawing; a rendered copy has to be asked for
        // separately, and is what the reader and the export actually show.
        this.pendingXML = typeof message.xml === 'string' ? message.xml : this.xml
        return { kind: 'export' }

      case 'export': {
        const svg = decodeExport(message.data)
        const xml = this.pendingXML ?? this.xml
        this.pendingXML = null
        if (!svg) {
          // Without a rendering there is nothing for a reader to look at, so
          // the drawing is not stored either: a half-saved diagram that shows
          // as broken would be worse than an unsaved one.
          return { kind: 'none' }
        }
        this.xml = xml
        return { kind: 'save', xml, svg }
      }

      case 'exit':
        return { kind: 'close' }

      default:
        return { kind: 'none' }
    }
  }

  /** The message to send for an action, or null when nothing is sent. */
  static request(action: DrawioAction): string | null {
    switch (action.kind) {
      case 'load':
        return JSON.stringify({ action: 'load', autosave: 0, xml: action.xml })
      case 'export':
        // An embedded SVG carries the drawing's source inside it, which is
        // what lets the diagram be reopened from the preview alone if its
        // source attachment is ever lost.
        return JSON.stringify({ action: 'export', format: 'xmlsvg', spin: 'Rendering', background: null })
      default:
        return null
    }
  }
}

/**
 * Turns the export message's payload into SVG text.
 *
 * draw.io answers with a data URI. Only an SVG one is accepted: the export was
 * asked for as SVG, so anything else means the editor did something other than
 * what was requested, and guessing at it would be how a different format ends
 * up stored under an .svg name.
 */
export function decodeExport(data: unknown): string {
  if (typeof data !== 'string' || data === '') return ''
  const match = /^data:image\/svg\+xml;base64,(.*)$/i.exec(data.trim())
  if (!match) return ''
  try {
    const binary = atob(match[1]!)
    const bytes = new Uint8Array(binary.length)
    for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
    return new TextDecoder().decode(bytes)
  } catch {
    return ''
  }
}

/** The empty drawing a new diagram starts from. */
export const EMPTY_DRAWIO_XML =
  '<mxfile><diagram id="new" name="Page-1">' +
  '<mxGraphModel><root><mxCell id="0"/><mxCell id="1" parent="0"/></root></mxGraphModel>' +
  '</diagram></mxfile>'
