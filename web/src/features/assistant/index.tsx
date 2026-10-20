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
import { useMutation, useQuery } from '@tanstack/react-query'
import { Send, Sparkles } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { api } from '@/lib/api'
import {
  getServerErrorMessage,
  requireServerSuccess,
} from '@/lib/server-error-message'

interface AssistantContext {
  account: Record<string, unknown>
  purchases: { order_id: number; product_name: string; status: number }[]
  tickets: { id: number; product_name: string; status: number }[]
}

interface ChatMessage {
  id: string
  role: 'user' | 'assistant'
  content: string
}

let messageSeq = 0
function nextMessageId(): string {
  messageSeq += 1
  return `m${messageSeq}`
}

async function fetchContext(): Promise<AssistantContext> {
  const response = await api.get('/api/store/assistant/context')
  return requireServerSuccess(response.data).data as AssistantContext
}

async function askAssistant(
  history: ChatMessage[],
  question: string
): Promise<{ reply: string; actionResult: string }> {
  const messages = [
    ...history.map((item) => ({ role: item.role, content: item.content })),
    { role: 'user', content: question },
  ]
  const response = await api.post('/api/store/assistant/chat', { messages })
  const payload = requireServerSuccess(response.data)
  const data = payload.data as { reply?: string; action_result?: string }
  return {
    reply: String(data?.reply ?? ''),
    actionResult: String(data?.action_result ?? ''),
  }
}

/** AI 客服：能查自己的余额/购买/售后，并按需创建售后单或投诉。 */
export function AssistantPage() {
  const { t } = useTranslation()
  const [input, setInput] = useState('')
  const [messages, setMessages] = useState<ChatMessage[]>([])

  const contextQuery = useQuery({
    queryKey: ['assistant-context'],
    queryFn: fetchContext,
  })

  const askMutation = useMutation({
    mutationFn: async (question: string) => {
      if (!contextQuery.data) throw new Error(t('Loading account data'))
      return askAssistant(messages, question)
    },
    onSuccess: (result, question) => {
      setMessages((current) => [
        ...current,
        { id: nextMessageId(), role: 'user', content: question },
        {
          id: nextMessageId(),
          role: 'assistant',
          content:
            result.reply === '' && result.actionResult === ''
              ? t('Done')
              : [result.reply, result.actionResult].filter(Boolean).join('\n'),
        },
      ])
      setInput('')
    },
    onError: (error) =>
      toast.error(getServerErrorMessage(error, t('Assistant failed'))),
  })

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('AI Support')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='grid gap-4'>
          <Card>
            <CardHeader className='gap-2'>
              <CardTitle className='flex items-center gap-2'>
                <Sparkles className='size-4' aria-hidden='true' />
                {t('Ask about your purchases, balance or after-sales')}
              </CardTitle>
            </CardHeader>
            <CardContent className='grid gap-3'>
              <div className='grid max-h-[26rem] gap-2 overflow-auto rounded-lg border p-3'>
                {messages.length === 0 && (
                  <p className='text-muted-foreground text-sm'>
                    {t('Try: 我买了某个商品，现在用不了，帮我看看')}
                  </p>
                )}
                {messages.map((message) => (
                  <div
                    key={message.id}
                    className={
                      message.role === 'user'
                        ? 'bg-muted ml-auto max-w-[80%] rounded-lg px-3 py-2 text-sm whitespace-pre-wrap'
                        : 'max-w-[85%] rounded-lg border px-3 py-2 text-sm whitespace-pre-wrap'
                    }
                  >
                    {message.content}
                  </div>
                ))}
              </div>
              <form
                className='flex gap-2'
                onSubmit={(event) => {
                  event.preventDefault()
                  const question = input.trim()
                  if (question === '' || askMutation.isPending) return
                  askMutation.mutate(question)
                }}
              >
                <Input
                  value={input}
                  placeholder={t('Describe what happened…')}
                  onChange={(event) => setInput(event.target.value)}
                />
                <Button
                  type='submit'
                  size='icon'
                  disabled={askMutation.isPending || input.trim() === ''}
                  aria-label={t('Send')}
                >
                  <Send className='size-4' aria-hidden='true' />
                </Button>
              </form>
            </CardContent>
          </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
