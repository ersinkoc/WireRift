import { motion } from 'framer-motion'
import { Download, Monitor, Apple, Terminal, Server, Cpu, ExternalLink } from 'lucide-react'
import { CodeBlock } from '@/components/ui/CodeBlock'
import { SITE } from '@/lib/constants'

const platforms = [
  {
    os: 'Linux',
    icon: Terminal,
    color: '#f59e0b',
    variants: [
      { arch: 'x64 (amd64)', client: 'wirerift-linux-amd64', server: 'wirerift-server-linux-amd64' },
      { arch: 'ARM64', client: 'wirerift-linux-arm64', server: 'wirerift-server-linux-arm64' },
      { arch: 'ARM (32-bit)', client: 'wirerift-linux-arm', server: 'wirerift-server-linux-arm' },
    ],
  },
  {
    os: 'macOS',
    icon: Apple,
    color: '#a78bfa',
    variants: [
      { arch: 'Apple Silicon (M1/M2/M3)', client: 'wirerift-darwin-arm64', server: 'wirerift-server-darwin-arm64' },
      { arch: 'Intel x64', client: 'wirerift-darwin-amd64', server: 'wirerift-server-darwin-amd64' },
    ],
  },
  {
    os: 'Windows',
    icon: Monitor,
    color: '#3b82f6',
    variants: [
      { arch: 'x64 (amd64)', client: 'wirerift-windows-amd64.zip', server: 'wirerift-server-windows-amd64.zip' },
      { arch: 'ARM64', client: 'wirerift-windows-arm64.zip', server: 'wirerift-server-windows-arm64.zip' },
    ],
  },
  {
    os: 'FreeBSD',
    icon: Cpu,
    color: '#ef4444',
    variants: [
      { arch: 'x64 (amd64)', client: 'wirerift-freebsd-amd64', server: 'wirerift-server-freebsd-amd64' },
    ],
  },
]

function getDownloadUrl(filename: string): string {
  return `https://github.com/wirerift/wirerift/releases/latest/download/${filename}`
}

export default function DownloadPage() {
  return (
    <div className="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 py-16 md:py-24">
      {/* Header */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5 }}
        className="text-center mb-16"
      >
        <h1 className="text-4xl md:text-5xl font-bold text-[var(--color-text-heading)]">
          Download <span className="gradient-text">WireRift</span>
        </h1>
        <p className="mt-4 text-lg text-[var(--color-text-muted)] max-w-2xl mx-auto">
          Pre-built binaries for every major platform. Or install with Go.
        </p>
      </motion.div>

      {/* Go Install */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5, delay: 0.1 }}
        className="mb-16"
      >
        <div className="p-6 rounded-2xl bg-[var(--color-bg-elevated)] border border-[var(--color-border)]">
          <div className="flex items-center gap-3 mb-4">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-cyan-500/20 to-blue-500/20 flex items-center justify-center">
              <Terminal className="w-5 h-5 text-cyan-400" />
            </div>
            <div>
              <h2 className="text-lg font-semibold text-[var(--color-text-heading)]">Install with Go</h2>
              <p className="text-sm text-[var(--color-text-muted)]">Requires Go 1.23+</p>
            </div>
          </div>
          <CodeBlock
            code={`# Install both client and server
go install github.com/wirerift/wirerift/cmd/wirerift@latest
go install github.com/wirerift/wirerift/cmd/wirerift-server@latest`}
            language="bash"
            filename="install.sh"
          />
        </div>
      </motion.div>

      {/* Build from Source */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5, delay: 0.15 }}
        className="mb-16"
      >
        <div className="p-6 rounded-2xl bg-[var(--color-bg-elevated)] border border-[var(--color-border)]">
          <div className="flex items-center gap-3 mb-4">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-green-500/20 to-emerald-500/20 flex items-center justify-center">
              <Server className="w-5 h-5 text-green-400" />
            </div>
            <div>
              <h2 className="text-lg font-semibold text-[var(--color-text-heading)]">Build from Source</h2>
              <p className="text-sm text-[var(--color-text-muted)]">Clone and compile yourself</p>
            </div>
          </div>
          <CodeBlock
            code={`git clone https://github.com/wirerift/wirerift.git
cd wirerift
make build

# Binaries are in ./bin/
./bin/wirerift version
./bin/wirerift-server -version`}
            language="bash"
            filename="build.sh"
          />
        </div>
      </motion.div>

      {/* Platform Downloads */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5, delay: 0.2 }}
        className="mb-12"
      >
        <h2 className="text-2xl font-bold text-[var(--color-text-heading)] mb-2">
          Pre-built Binaries
        </h2>
        <p className="text-[var(--color-text-muted)] mb-8">
          Download from the{' '}
          <a
            href={`${SITE.repo}/releases/latest`}
            target="_blank"
            rel="noopener noreferrer"
            className="text-[var(--color-primary-500)] hover:underline inline-flex items-center gap-1"
          >
            latest GitHub release <ExternalLink className="w-3 h-3" />
          </a>
        </p>
      </motion.div>

      <div className="space-y-6">
        {platforms.map((platform, i) => {
          const Icon = platform.icon
          return (
            <motion.div
              key={platform.os}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.4, delay: 0.25 + i * 0.05 }}
              className="rounded-2xl bg-[var(--color-bg-elevated)] border border-[var(--color-border)] overflow-hidden"
            >
              {/* Platform header */}
              <div className="px-6 py-4 flex items-center gap-3 border-b border-[var(--color-border)]">
                <Icon className="w-5 h-5" style={{ color: platform.color }} />
                <h3 className="text-lg font-semibold text-[var(--color-text-heading)]">{platform.os}</h3>
              </div>

              {/* Variants table */}
              <div className="divide-y divide-[var(--color-border)]">
                {platform.variants.map((v) => (
                  <div key={v.arch} className="px-6 py-3 flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4">
                    <span className="text-sm text-[var(--color-text-muted)] sm:w-48 shrink-0">
                      {v.arch}
                    </span>
                    <div className="flex flex-wrap gap-2">
                      <a
                        href={getDownloadUrl(v.client)}
                        className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-blue-500/10 text-blue-400 border border-blue-500/20 hover:bg-blue-500/20 transition-colors"
                      >
                        <Download className="w-3 h-3" />
                        Client
                      </a>
                      <a
                        href={getDownloadUrl(v.server)}
                        className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-violet-500/10 text-violet-400 border border-violet-500/20 hover:bg-violet-500/20 transition-colors"
                      >
                        <Download className="w-3 h-3" />
                        Server
                      </a>
                    </div>
                  </div>
                ))}
              </div>
            </motion.div>
          )
        })}
      </div>

      {/* Verify */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true }}
        transition={{ duration: 0.5 }}
        className="mt-12"
      >
        <div className="p-6 rounded-2xl bg-[var(--color-bg-elevated)] border border-[var(--color-border)]">
          <h3 className="text-lg font-semibold text-[var(--color-text-heading)] mb-3">
            Verify Checksums
          </h3>
          <p className="text-sm text-[var(--color-text-muted)] mb-4">
            Each release includes a <code className="px-1.5 py-0.5 rounded bg-[var(--color-bg-code)] text-xs font-mono">checksums.txt</code> file with SHA-256 hashes.
          </p>
          <CodeBlock
            code={`# Download checksums
curl -LO https://github.com/wirerift/wirerift/releases/latest/download/checksums.txt

# Verify your download
sha256sum -c checksums.txt --ignore-missing`}
            language="bash"
            filename="verify.sh"
          />
        </div>
      </motion.div>
    </div>
  )
}
