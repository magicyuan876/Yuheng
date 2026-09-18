import { loadConfig } from './config.js'
import { createLogger } from './log.js'
import { createCollabServer } from './server.js'

async function main(): Promise<void> {
  let cfg
  try {
    cfg = loadConfig()
  } catch (err) {
    process.stderr.write(`collab: ${(err as Error).message}\n`)
    process.exit(2)
  }
  const log = createLogger(cfg.logLevel, { service: 'yuheng-collab' })
  const collab = createCollabServer(cfg, log)
  await collab.start()

  let shuttingDown = false
  const shutdown = (signal: string) => {
    if (shuttingDown) return
    shuttingDown = true
    log.info('signal received', { signal })
    collab.stop().then(() => process.exit(0), (err) => {
      log.error('shutdown failed', { error: (err as Error).message })
      process.exit(1)
    })
  }
  process.on('SIGTERM', () => shutdown('SIGTERM'))
  process.on('SIGINT', () => shutdown('SIGINT'))
  process.on('unhandledRejection', (reason) => {
    log.error('unhandled rejection', { error: reason instanceof Error ? reason.message : String(reason) })
  })
}

void main()
