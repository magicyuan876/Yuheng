import type { LogLevel } from './config.js'

const ORDER: Record<LogLevel, number> = { debug: 10, info: 20, warn: 30, error: 40 }

export interface Logger {
  debug(msg: string, fields?: Record<string, unknown>): void
  info(msg: string, fields?: Record<string, unknown>): void
  warn(msg: string, fields?: Record<string, unknown>): void
  error(msg: string, fields?: Record<string, unknown>): void
  child(fields: Record<string, unknown>): Logger
}

/** One JSON object per line on stdout, the shape log collectors expect. */
export function createLogger(level: LogLevel, base: Record<string, unknown> = {}, out = process.stdout): Logger {
  const min = ORDER[level]
  const write = (lvl: LogLevel, msg: string, fields?: Record<string, unknown>) => {
    if (ORDER[lvl] < min) return
    const line = { time: new Date().toISOString(), level: lvl, msg, ...base, ...(fields ?? {}) }
    out.write(`${JSON.stringify(line, replacer)}\n`)
  }
  return {
    debug: (m, f) => write('debug', m, f),
    info: (m, f) => write('info', m, f),
    warn: (m, f) => write('warn', m, f),
    error: (m, f) => write('error', m, f),
    child: (fields) => createLogger(level, { ...base, ...fields }, out),
  }
}

function replacer(_key: string, value: unknown): unknown {
  if (value instanceof Error) return { name: value.name, message: value.message }
  if (typeof value === 'bigint') return value.toString()
  return value
}

/** A logger that drops everything (tests). */
export const silentLogger: Logger = {
  debug() {}, info() {}, warn() {}, error() {}, child() { return silentLogger },
}
