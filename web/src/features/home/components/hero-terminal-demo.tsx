/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Pause, Play } from 'lucide-react'
import { useState, useEffect, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { cn } from '@/lib/utils'

type AccentTone = 'emerald' | 'amber' | 'blue' | 'violet'

interface ApiDemoConfig {
  id: string
  label: string
  method: 'POST' | 'GET'
  endpoint: string
  headers: string[]
  request: string[]
  response: string[]
  responseHighlights: string[]
  tokens: number
  latency: number
  accent: AccentTone
}

const ACCENT_CLASSES: Record<
  AccentTone,
  {
    activeText: string
    activeBorder: string
    badge: string
  }
> = {
  emerald: {
    activeText: 'text-success',
    activeBorder: 'border-success',
    badge: 'bg-success/10 text-success',
  },
  amber: {
    activeText: 'text-warning',
    activeBorder: 'border-warning',
    badge: 'bg-warning/10 text-warning',
  },
  blue: {
    activeText: 'text-primary',
    activeBorder: 'border-primary',
    badge: 'bg-primary/10 text-primary',
  },
  violet: {
    activeText: 'text-chart-3',
    activeBorder: 'border-chart-3',
    badge: 'bg-chart-3/10 text-chart-3',
  },
}

const API_DEMOS: ApiDemoConfig[] = [
  {
    id: 'gpt-chat',
    label: 'Chat',
    method: 'POST',
    endpoint: '/v1/chat/completions',
    headers: ['"Authorization: Bearer sk-••••"'],
    request: [
      '"model": "your-model",',
      '"messages": [',
      '  { "role": "user", "content": "..." }',
      ']',
    ],
    response: [
      '{',
      '  "choices": [{ "message": { "content": <text> } }],',
      '  "usage": { "total_tokens": <tokens> }',
      '}',
    ],
    responseHighlights: ['<text>', '<tokens>'],
    tokens: 27,
    latency: 142,
    accent: 'emerald',
  },
  {
    id: 'responses',
    label: 'Responses',
    method: 'POST',
    endpoint: '/v1/responses',
    headers: ['"Authorization: Bearer sk-••••"'],
    request: ['"model": "your-model",', '"input": "..."'],
    response: [
      '{',
      '  "output": [{ "type": "output_text", "text": <text> }],',
      '  "usage": { "total_tokens": <tokens> }',
      '}',
    ],
    responseHighlights: ['<text>', '<tokens>'],
    tokens: 31,
    latency: 168,
    accent: 'amber',
  },
  {
    id: 'claude',
    label: 'Claude',
    method: 'POST',
    endpoint: '/v1/messages',
    headers: ['"x-api-key: sk-••••"', '"anthropic-version: 2023-06-01"'],
    request: [
      '"model": "your-model",',
      '"max_tokens": 1024,',
      '"messages": [',
      '  { "role": "user", "content": "..." }',
      ']',
    ],
    response: [
      '{',
      '  "content": [{ "type": "text", "text": <text> }],',
      '  "usage": { "input_tokens": <in>, "output_tokens": <out> }',
      '}',
    ],
    responseHighlights: ['<text>', '<in>', '<out>'],
    tokens: 29,
    latency: 156,
    accent: 'blue',
  },
  {
    id: 'gemini',
    label: 'Gemini',
    method: 'POST',
    endpoint: '/v1beta/models/{model}:generateContent',
    headers: ['"x-goog-api-key: sk-••••"'],
    request: [
      '"contents": [',
      '  { "role": "user",',
      '    "parts": [{ "text": "..." }] }',
      ']',
    ],
    response: [
      '{',
      '  "candidates": [{ "content": { "parts": [{ "text": <text> }] } }],',
      '  "usageMetadata": { "totalTokenCount": <tokens> }',
      '}',
    ],
    responseHighlights: ['<text>', '<tokens>'],
    tokens: 25,
    latency: 93,
    accent: 'violet',
  },
]

const CYCLE_INTERVAL = 4500

interface HeroTerminalDemoProps {
  className?: string
}

export function HeroTerminalDemo(props: HeroTerminalDemoProps) {
  const { t } = useTranslation()
  const [activeIndex, setActiveIndex] = useState(0)
  const [isPlaying, setIsPlaying] = useState(false)

  useEffect(() => {
    if (!isPlaying) return

    const cycleTimer = setTimeout(() => {
      setActiveIndex((prev) => (prev + 1) % API_DEMOS.length)
    }, CYCLE_INTERVAL)

    return () => {
      clearTimeout(cycleTimer)
    }
  }, [activeIndex, isPlaying])

  const handleSelect = (index: number) => {
    setActiveIndex(index)
    setIsPlaying(false)
  }

  const handleTogglePlayback = () => {
    setIsPlaying((prev) => !prev)
  }

  const demo = API_DEMOS[activeIndex]
  const accent = ACCENT_CLASSES[demo.accent]

  return (
    <div className={cn('mx-auto min-w-0 w-full max-w-2xl', props.className)}>
      <Tabs
        value={activeIndex}
        onValueChange={(value) => {
          if (typeof value === 'number') handleSelect(value)
        }}
        className={cn(
          'border-border bg-card gap-0 overflow-hidden rounded-2xl border',
          'shadow-hero'
        )}
      >
        {/* Tab strip */}
        <div
          className={cn(
            'flex min-w-0 items-center gap-2 border-b px-3 py-2 sm:px-4',
            'border-border/70 bg-muted/40'
          )}
        >
          <div className='min-w-0 flex-1 overflow-x-auto'>
            <TabsList
              activateOnFocus
              aria-label={t('API')}
              className='h-9 justify-start gap-0.5 rounded-lg bg-transparent p-0'
            >
              {API_DEMOS.map((item, index) => {
                const tone = ACCENT_CLASSES[item.accent]
                const isActive = index === activeIndex
                return (
                  <TabsTrigger
                    key={item.id}
                    value={index}
                    className={cn(
                      'h-9 flex-none rounded-lg px-2.5 text-xs sm:px-3',
                      isActive ? tone.activeText : 'text-muted-foreground'
                    )}
                  >
                    {item.label}
                  </TabsTrigger>
                )
              })}
            </TabsList>
          </div>
          <div className='flex shrink-0 items-center gap-2'>
            <Button
              type='button'
              variant='ghost'
              size='icon-sm'
              onClick={handleTogglePlayback}
              aria-label={isPlaying ? t('Stop') : t('Start')}
              className='text-muted-foreground hover:text-foreground'
            >
              {isPlaying ? (
                <Pause aria-hidden className='size-3.5' />
              ) : (
                <Play aria-hidden className='size-3.5' />
              )}
            </Button>
            <span
              aria-hidden
              className='bg-success hidden size-1.5 rounded-full sm:inline-block'
            />
            <span className='text-muted-foreground hidden font-mono text-[10px] tracking-wider uppercase sm:inline'>
              200 ok
            </span>
          </div>
        </div>

        <TabsContent value={activeIndex} className='min-w-0'>
          {/* Endpoint row */}
          <div
            className={cn(
              'flex min-w-0 items-center gap-2.5 border-b px-4 py-2 sm:px-5',
              'border-border/40'
            )}
          >
            <span
              className={cn(
                'rounded-md px-1.5 py-0.5 font-mono text-[10px] font-semibold tracking-wider',
                accent.badge
              )}
            >
              {demo.method}
            </span>
            <code
              className={cn(
                'text-foreground min-w-0 flex-1 truncate font-mono text-xs'
              )}
            >
              {demo.endpoint}
            </code>
            <CopyButton
              value={demo.endpoint}
              tooltip={t('Copy to clipboard')}
              className='size-8'
              iconClassName='size-3.5'
            />
          </div>

          {/* Body — fixed rows so neither block shifts when switching demos */}
          <div className='grid h-[400px] min-w-0 grid-rows-[235px_minmax(0,1fr)] font-mono text-xs leading-[1.65] sm:text-[12.5px]'>
            {/* Request */}
            <RequestBlock demo={demo} />

            {/* Response */}
            <ResponseBlock demo={demo} />
          </div>
        </TabsContent>

        {/* Footer metrics */}
        <div
          className={cn(
            'flex items-center justify-between border-t px-5 py-2.5',
            'border-border/40 bg-muted/30'
          )}
        >
          <div className='text-muted-foreground flex items-center gap-3 text-[10px] tabular-nums'>
            <span className='flex items-center gap-1'>
              <span className='font-mono'>{demo.latency}</span>
              <span className='tracking-wider uppercase'>ms</span>
            </span>
            <span className='bg-foreground/15 size-1 rounded-full' />
            <span className='flex items-center gap-1'>
              <span className='font-mono'>{demo.tokens}</span>
              <span className='tracking-wider uppercase'>tokens</span>
            </span>
            <span className='bg-foreground/15 size-1 rounded-full' />
            <span className='flex items-center gap-1'>
              <span className='tracking-wider uppercase'>cost</span>
              <span className='font-mono'>
                ${(demo.tokens * 0.00003).toFixed(5)}
              </span>
            </span>
          </div>
          <span className='text-muted-foreground hidden font-mono text-[10px] tracking-wider uppercase sm:inline'>
            stream · sse
          </span>
        </div>
      </Tabs>
    </div>
  )
}

function RequestBlock(props: { demo: ApiDemoConfig }) {
  const { demo } = props
  const { t } = useTranslation()

  return (
    <div className='relative min-w-0 overflow-x-auto px-4 py-4 sm:px-5'>
      <SectionLabel>{t('Request')}</SectionLabel>
      <div className='mt-2'>
        <CodeLine>
          <Command>curl</Command> <Flag>-X</Flag> <Flag>POST</Flag>{' '}
          <StringText>&quot;{demo.endpoint}&quot;</StringText>{' '}
          <Muted>{'\\'}</Muted>
        </CodeLine>
        {demo.headers.map((header) => (
          <CodeLine key={header} indent={2}>
            <Flag>-H</Flag> <StringText>{header}</StringText>{' '}
            <Muted>{'\\'}</Muted>
          </CodeLine>
        ))}
        <CodeLine indent={2}>
          <Flag>-d</Flag> <StringText>&apos;{'{'}</StringText>
        </CodeLine>
        {demo.request.map((line) => (
          <CodeLine key={line} indent={4}>
            {renderJsonLine(line)}
          </CodeLine>
        ))}
        <CodeLine indent={2}>
          <StringText>{'}'}&apos;</StringText>
        </CodeLine>
      </div>
    </div>
  )
}

function ResponseBlock(props: { demo: ApiDemoConfig }) {
  const { demo } = props
  const { t } = useTranslation()

  return (
    <div
      className={cn(
        'relative min-w-0 overflow-x-auto border-t px-4 py-4 sm:px-5',
        'border-border/40 bg-muted/20'
      )}
    >
      <SectionLabel>{t('Response')}</SectionLabel>
      <div className='mt-2'>
        {demo.response.map((line) => (
          <CodeLine key={line}>{renderResponseLine(line, demo)}</CodeLine>
        ))}
      </div>
    </div>
  )
}

function SectionLabel(props: { children: ReactNode }) {
  return (
    <span className='text-muted-foreground font-sans text-[10px] font-semibold tracking-[0.14em] uppercase'>
      {props.children}
    </span>
  )
}

const STRING_RE = /"[^"]*"/g
const PLACEHOLDER_RE = /<[a-z]+>/gi

function renderJsonLine(line: string): ReactNode {
  if (!line.trim()) return <Muted> </Muted>
  return tokenize(line)
}

function renderResponseLine(line: string, demo: ApiDemoConfig): ReactNode {
  if (!line.trim()) return <Muted> </Muted>

  const segments: ReactNode[] = []
  let cursor = 0
  const matches = [...line.matchAll(PLACEHOLDER_RE)]

  if (matches.length === 0) return tokenize(line)

  matches.forEach((match) => {
    const start = match.index ?? 0
    if (start > cursor) {
      segments.push(
        <span key={`pre-${start}`}>{tokenize(line.slice(cursor, start))}</span>
      )
    }
    const placeholder = match[0]
    if (placeholder === '<text>') {
      segments.push(
        <Accent key={`ph-${start}`} accent={demo.accent}>
          {`"${truncateResponse(demo)}"`}
        </Accent>
      )
    } else if (placeholder === '<tokens>') {
      segments.push(<NumberText key={`ph-${start}`}>{demo.tokens}</NumberText>)
    } else if (placeholder === '<in>') {
      segments.push(
        <NumberText key={`ph-${start}`}>
          {Math.floor(demo.tokens * 0.4)}
        </NumberText>
      )
    } else if (placeholder === '<out>') {
      segments.push(
        <NumberText key={`ph-${start}`}>
          {Math.ceil(demo.tokens * 0.6)}
        </NumberText>
      )
    } else {
      segments.push(<Muted key={`ph-${start}`}>{placeholder}</Muted>)
    }
    cursor = start + placeholder.length
  })

  if (cursor < line.length) {
    segments.push(<span key='tail'>{tokenize(line.slice(cursor))}</span>)
  }

  return segments
}

function truncateResponse(demo: ApiDemoConfig): string {
  const map: Record<string, string> = {
    'gpt-chat': 'Chat request routed.',
    responses: 'Response workflow ready.',
    claude: 'Claude message routed.',
    gemini: 'Gemini request served.',
  }
  return map[demo.id] ?? '...'
}

function tokenize(input: string): ReactNode {
  // Split string into "..." string runs and the rest, then color keys/punct.
  const segments: ReactNode[] = []
  let cursor = 0
  const matches = [...input.matchAll(STRING_RE)]

  matches.forEach((match) => {
    const start = match.index ?? 0
    if (start > cursor) {
      segments.push(
        <Muted key={`m-${start}`}>{input.slice(cursor, start)}</Muted>
      )
    }
    const text = match[0]
    const after = input.slice(start + text.length).trimStart()
    const isKey = after.startsWith(':')
    if (isKey) {
      segments.push(<Key key={`k-${start}`}>{text}</Key>)
    } else {
      segments.push(<StringText key={`s-${start}`}>{text}</StringText>)
    }
    cursor = start + text.length
  })

  if (cursor < input.length) {
    segments.push(<Muted key='tail'>{input.slice(cursor)}</Muted>)
  }

  return segments
}

function CodeLine(props: { children: ReactNode; indent?: number }) {
  return (
    <div className='whitespace-pre'>
      {props.indent ? (
        <span
          aria-hidden
          className='inline-block'
          style={{ width: `${props.indent}ch` }}
        />
      ) : null}
      {props.children}
    </div>
  )
}

function Command(props: { children: ReactNode }) {
  return <span className='text-success font-medium'>{props.children}</span>
}

function Flag(props: { children: ReactNode }) {
  return <span className='text-primary'>{props.children}</span>
}

function Key(props: { children: ReactNode }) {
  return <span className='text-chart-2'>{props.children}</span>
}

function StringText(props: { children: ReactNode }) {
  return <span className='text-warning'>{props.children}</span>
}

function NumberText(props: { children: ReactNode }) {
  return <span className='text-chart-3 font-medium'>{props.children}</span>
}

function Muted(props: { children: ReactNode }) {
  return <span className='text-foreground/55'>{props.children}</span>
}

function Accent(props: { children: ReactNode; accent: AccentTone }) {
  const tone = ACCENT_CLASSES[props.accent]
  return (
    <span className={cn('font-medium', tone.activeText)}>{props.children}</span>
  )
}
