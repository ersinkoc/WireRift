import { Link } from 'react-router'
import { motion } from 'framer-motion'
import { ArrowRight, Github, Terminal, Zap, Shield, Box, TestTube } from 'lucide-react'
import { CodeBlock } from '@/components/ui/CodeBlock'
import { CopyButton } from '@/components/ui/CopyButton'
import { SITE } from '@/lib/constants'

const stagger = {
  hidden: { opacity: 0 },
  visible: { opacity: 1, transition: { staggerChildren: 0.08, delayChildren: 0.15 } },
} as const

const fadeUp = {
  hidden: { opacity: 0, y: 24 },
  visible: {
    opacity: 1, y: 0,
    transition: { duration: 0.6, ease: [0.25, 0.46, 0.45, 0.94] as const },
  },
}

const codeSlide = {
  hidden: { opacity: 0, y: 32, scale: 0.97 },
  visible: {
    opacity: 1, y: 0, scale: 1,
    transition: { duration: 0.7, ease: [0.25, 0.46, 0.45, 0.94] as const, delay: 0.3 },
  },
}

const heroCode = `# 1. Start your tunnel server
wirerift-server -domain mytunnel.com -auto-cert -v

# 2. Expose any local service to the internet
wirerift http 8080 myapp
# => https://myapp.mytunnel.com

# 3. Or forward raw TCP (databases, games, SSH)
wirerift tcp 5432`

const stats = [
  { icon: Zap, label: 'Zero Deps', desc: 'stdlib only' },
  { icon: Box, label: 'Single Binary', desc: 'no runtime' },
  { icon: Shield, label: 'Auto TLS', desc: 'out of the box' },
  { icon: TestTube, label: '100% Tested', desc: 'every line' },
]

export function Hero() {
  const installCmd = SITE.installCommand

  return (
    <section className="relative overflow-hidden">
      {/* Layered background */}
      <div className="hero-gradient-bg absolute inset-0" />
      <div className="hero-grid absolute inset-0" />

      {/* Glow orbs */}
      <div className="absolute top-1/4 left-1/4 w-[500px] h-[500px] bg-blue-500/8 rounded-full blur-[120px] pointer-events-none" />
      <div className="absolute bottom-1/4 right-1/4 w-[400px] h-[400px] bg-violet-500/8 rounded-full blur-[100px] pointer-events-none" />

      {/* Content */}
      <div className="relative max-w-6xl mx-auto px-5 sm:px-8 pt-24 pb-16 md:pt-32 md:pb-24 lg:pt-40 lg:pb-32">
        <motion.div variants={stagger} initial="hidden" animate="visible">

          {/* Version pill */}
          <motion.div variants={fadeUp} className="flex justify-center mb-8">
            <span className="inline-flex items-center gap-1.5 px-3.5 py-1.5 rounded-full text-xs font-semibold tracking-wide border border-[var(--color-border)] bg-[var(--color-bg-elevated)] text-[var(--color-text-muted)] backdrop-blur-sm">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
              v{SITE.version} — Now Available
            </span>
          </motion.div>

          {/* Headline */}
          <motion.h1 variants={fadeUp} className="text-center">
            <span className="block text-4xl sm:text-5xl md:text-6xl lg:text-7xl font-extrabold tracking-tight text-[var(--color-text-heading)]">
              Expose localhost
            </span>
            <span className="block mt-1 sm:mt-2 text-4xl sm:text-5xl md:text-6xl lg:text-7xl font-extrabold tracking-tight gradient-text">
              to the world.
            </span>
          </motion.h1>

          {/* Subtitle */}
          <motion.p variants={fadeUp} className="mt-5 md:mt-6 text-center text-base sm:text-lg md:text-xl text-[var(--color-text-muted)] max-w-xl mx-auto leading-relaxed">
            Self-hosted tunnel server with zero dependencies.
            HTTP &amp; TCP tunnels, auto TLS, stream multiplexing — all in a single Go binary.
          </motion.p>

          {/* CTA row */}
          <motion.div variants={fadeUp} className="mt-8 md:mt-10 flex flex-col sm:flex-row items-center justify-center gap-3">
            <Link to="/docs/getting-started" className="w-full sm:w-auto">
              <button className="w-full sm:w-auto inline-flex items-center justify-center gap-2 px-7 py-3 text-base font-semibold rounded-xl text-white bg-gradient-to-r from-blue-500 to-violet-500 hover:from-blue-600 hover:to-violet-600 shadow-lg shadow-blue-500/20 hover:shadow-blue-500/30 transition-all duration-200 cursor-pointer">
                Get Started
                <ArrowRight className="w-4 h-4" />
              </button>
            </Link>
            <a href={SITE.repo} target="_blank" rel="noopener noreferrer" className="w-full sm:w-auto">
              <button className="w-full sm:w-auto inline-flex items-center justify-center gap-2 px-7 py-3 text-base font-semibold rounded-xl text-[var(--color-text-heading)] border border-[var(--color-border)] hover:bg-[var(--color-bg-tertiary)] transition-all duration-200 cursor-pointer">
                <Github className="w-4 h-4" />
                GitHub
              </button>
            </a>
          </motion.div>

          {/* Install command */}
          <motion.div variants={fadeUp} className="mt-5 flex justify-center">
            <div className="inline-flex items-center gap-2.5 px-4 py-2.5 rounded-xl bg-[var(--color-bg-code)] border border-[var(--color-border)] font-mono text-sm backdrop-blur-sm max-w-full overflow-hidden">
              <Terminal className="w-4 h-4 text-[var(--color-text-muted)] shrink-0" />
              <code className="text-[var(--color-text-muted)] truncate">{installCmd}</code>
              <CopyButton text={installCmd} />
            </div>
          </motion.div>

          {/* Stats row */}
          <motion.div variants={fadeUp} className="mt-10 md:mt-12 grid grid-cols-2 sm:grid-cols-4 gap-3 max-w-2xl mx-auto">
            {stats.map((s) => {
              const Icon = s.icon
              return (
                <div key={s.label} className="flex flex-col items-center gap-1.5 p-3 rounded-xl bg-[var(--color-bg-elevated)]/50 border border-[var(--color-border)]/50 backdrop-blur-sm">
                  <Icon className="w-4 h-4 text-blue-400" />
                  <span className="text-xs font-semibold text-[var(--color-text-heading)]">{s.label}</span>
                  <span className="text-[10px] text-[var(--color-text-muted)]">{s.desc}</span>
                </div>
              )
            })}
          </motion.div>

          {/* Code block */}
          <motion.div variants={codeSlide} className="mt-12 md:mt-16 max-w-2xl mx-auto">
            <div className="relative rounded-2xl overflow-hidden ring-1 ring-[var(--color-border)] shadow-2xl shadow-black/20">
              {/* Glow behind code */}
              <div className="absolute -inset-1 bg-gradient-to-r from-blue-500/10 via-violet-500/10 to-blue-500/10 blur-xl rounded-2xl pointer-events-none" />
              <div className="relative">
                <CodeBlock
                  code={heroCode}
                  language="bash"
                  filename="terminal"
                  lineNumbers={true}
                  showLanguageBadge={false}
                />
              </div>
            </div>
          </motion.div>

        </motion.div>
      </div>
    </section>
  )
}
