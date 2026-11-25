import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Play, Plus, RefreshCw, X } from 'lucide-react'
import { nanoid } from 'nanoid'
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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { SectionPageLayout } from '@/components/layout'
import { LoadingState } from '@/components/loading-state'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Combobox } from '@/components/ui/combobox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Progress, ProgressLabel } from '@/components/ui/progress'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { toIntlLocale } from '@/i18n/languages'
import { hasPermission } from '@/lib/admin-permissions'
import { api } from '@/lib/api'
import { formatNumber, formatPercent } from '@/lib/format'
import { ROLE } from '@/lib/roles'
import {
  getServerErrorMessage,
  requireServerSuccess,
} from '@/lib/server-error-message'
import { useAuthStore } from '@/stores/auth-store'

type Target = { group: string; model: string; reference: string }
type TargetDraft = Target & { rowId: string }
type Schedule = { enabled: boolean; minutes: number }
type Config = {
  schedule: Schedule
  catalog: Record<string, string[]>
  targets: Target[]
  max_targets: number
  base_url: string
  last_run: Run | null
}
type Verdict =
  | 'match'
  | 'mismatch'
  | 'inconclusive'
  | 'reference_missing'
  | 'unavailable'
type Result = Target & {
  channel_id?: number
  channel_name?: string
  reported_model?: string
  checked_at?: string
  reference_built_at?: string
  verdict: Verdict
  failed_samples: number
  analysis?: {
    decision: 'scored' | 'partial' | 'unscorable' | 'not_model'
    prediction: string
    prediction_name: string
    family_prediction_name: string
    probability: number | null
    used_outputs: number
    diagnostics: {
      index: number
      accepted: boolean
      parsed_numbers: number
      minimum_numbers: number
    }[]
    results: {
      model: string
      display_name: string
      family_name: string
      score: number
      probability: number | null
    }[]
  }
}
type Run = { checked_at: string; reference_built_at: string; results: Result[] }
type ApiResult<T> = { success: boolean; data: T }
const path = '/api/model-fingerprint'

export function ModelFingerprint() {
  const { t, i18n } = useTranslation()
  const queryClient = useQueryClient()
  const user = useAuthStore((state) => state.auth.user)
  const admin =
    !!user && user.role >= ROLE.ADMIN && hasPermission(user, 'channel', 'read')
  const canWrite =
    admin &&
    hasPermission(user, 'channel', 'write') &&
    hasPermission(user, 'channel', 'operate')
  const canRun = admin && hasPermission(user, 'channel', 'operate')
  const configKey = ['model-fingerprint-config', user?.id]
  const published = useQuery({
    queryKey: ['model-fingerprint-results', user?.id],
    enabled: !!user && !admin,
    refetchInterval: 60000,
    queryFn: async ({ signal }) =>
      requireServerSuccess(
        (await api.get<ApiResult<Run | null>>(`${path}/results`, { signal }))
          .data
      ).data,
  })
  const [draft, setDraft] = useState<TargetDraft[] | null>(null)
  const [timerDraft, setTimerDraft] = useState<Schedule | null>(null)
  const [detail, setDetail] = useState<Result | null>(null)
  const [search, setSearch] = useState('')
  const [verdictFilter, setVerdictFilter] = useState('all')
  const [runGroup, setRunGroup] = useState('')
  const config = useQuery({
    queryKey: configKey,
    enabled: admin,
    refetchInterval: 60000,
    queryFn: async ({ signal }) =>
      requireServerSuccess(
        (await api.get<ApiResult<Config>>(path, { signal })).data
      ).data,
  })
  const save = useMutation({
    mutationFn: async (targets: Target[]) =>
      requireServerSuccess(
        (await api.put<ApiResult<Target[]>>(path, { targets, schedule })).data
      ).data,
    onSuccess: (targets) => {
      queryClient.setQueryData<Config>(
        configKey,
        (previous) => previous && { ...previous, targets }
      )
      void queryClient.invalidateQueries({ queryKey: configKey })
      setTimerDraft(null)
      setDraft(null)
    },
  })
  const run = useMutation({
    mutationFn: async (group: string) =>
      requireServerSuccess(
        (
          await api.post<ApiResult<Run>>(
            `${path}/run`,
            group ? { group } : {},
            { timeout: 520_000 }
          )
        ).data
      ).data,
    onSuccess: (last_run) =>
      queryClient.setQueryData<Config>(
        configKey,
        (previous) => previous && { ...previous, last_run }
      ),
  })
  const latest = admin ? config.data?.last_run : published.data
  const savedTargets = useMemo(
    () =>
      config.data?.targets.map((target) => ({
        ...target,
        rowId: JSON.stringify([target.group, target.model]),
      })) ?? [],
    [config.data?.targets]
  )
  const targets = draft ?? savedTargets
  const savedGroups = [
    ...new Set(savedTargets.map((target) => target.group)),
  ].sort()
  const catalog = config.data?.catalog ?? {}
  const groups = Object.keys(catalog)
    .sort()
    .map((name) => ({ value: name, label: name }))
  const schedule = timerDraft ??
    config.data?.schedule ?? { enabled: false, minutes: 10 }
  const dirty = draft !== null || timerDraft !== null
  const valid =
    Number.isInteger(schedule.minutes) &&
    schedule.minutes >= 1 &&
    schedule.minutes <= 1440 &&
    (!schedule.enabled || targets.length > 0) &&
    targets.every((target) => catalog[target.group]?.includes(target.model)) &&
    new Set(targets.map(({ group, model }) => JSON.stringify([group, model])))
      .size === targets.length
  const busy = save.isPending || run.isPending
  const resultsQuery = admin ? config : published
  const error = save.error ?? run.error ?? (latest ? resultsQuery.error : null)
  const results = latest?.results ?? []
  const normalizedSearch = search.trim().toLowerCase()
  const filteredResults = results.filter(
    (result) =>
      (verdictFilter === 'all' || result.verdict === verdictFilter) &&
      [
        result.group,
        result.model,
        result.reference,
        result.analysis?.prediction_name,
        result.analysis?.prediction,
      ].some((value) => value?.toLowerCase().includes(normalizedSearch))
  )
  const hasFilters = search !== '' || verdictFilter !== 'all'
  const locale = toIntlLocale(i18n.language)

  return (
    <SectionPageLayout stackActionsOnMobile>
      <SectionPageLayout.Title>
        {t('Model Fingerprinting')}
      </SectionPageLayout.Title>
      {canRun && (
        <SectionPageLayout.Actions>
          <NativeSelect
            aria-label={t('Group to test')}
            value={runGroup}
            disabled={busy || savedGroups.length === 0}
            onChange={(event) => setRunGroup(event.target.value)}
            className='w-full sm:max-w-64'
          >
            <NativeSelectOption value=''>
              {t('All configured groups')}
            </NativeSelectOption>
            {savedGroups.map((group) => (
              <NativeSelectOption key={group} value={group}>
                {group}
              </NativeSelectOption>
            ))}
          </NativeSelect>
          <Button
            onClick={() => run.mutate(runGroup)}
            disabled={
              busy ||
              dirty ||
              !valid ||
              targets.length === 0 ||
              (runGroup !== '' && !savedGroups.includes(runGroup))
            }
          >
            <Play data-icon='inline-start' />
            {run.isPending && t('Detecting models…')}
            {!run.isPending &&
              (runGroup
                ? t('Test selected group')
                : t('Run fingerprint check'))}
          </Button>
        </SectionPageLayout.Actions>
      )}
      <SectionPageLayout.Content>
        <div className='mx-auto grid max-w-6xl gap-5 py-2'>
          {error && (
            <Alert variant='destructive'>
              <AlertDescription>
                {getServerErrorMessage(error)}
              </AlertDescription>
            </Alert>
          )}
          <Card>
            <CardHeader className='flex flex-row flex-wrap items-center justify-between gap-3'>
              <CardTitle>{t('Fingerprint results')}</CardTitle>
              <Button
                size='sm'
                variant='outline'
                disabled={busy || resultsQuery.isFetching}
                onClick={() => void resultsQuery.refetch()}
              >
                <RefreshCw
                  aria-hidden='true'
                  className={
                    resultsQuery.isFetching
                      ? 'animate-spin motion-reduce:animate-none'
                      : undefined
                  }
                />
                {t('Refresh')}
              </Button>
            </CardHeader>
            <CardContent className='space-y-3'>
              {run.isPending && (
                <p className='text-muted-foreground text-sm' role='status'>
                  {run.variables
                    ? t('Detecting group {{group}}…', { group: run.variables })
                    : t('Waiting for model samples…')}
                </p>
              )}
              {resultsQuery.isPending && <LoadingState />}
              {resultsQuery.isError && !latest && (
                <ErrorState
                  description={getServerErrorMessage(resultsQuery.error)}
                  action={
                    <Button
                      variant='outline'
                      disabled={resultsQuery.isFetching}
                      onClick={() => void resultsQuery.refetch()}
                    >
                      {t('Retry')}
                    </Button>
                  }
                />
              )}
              {!resultsQuery.isPending &&
                !resultsQuery.isError &&
                !run.isPending &&
                (!latest || results.length === 0) && (
                  <EmptyState
                    title={t('No fingerprint results yet.')}
                    className='min-h-40'
                  />
                )}
              {latest && (
                <>
                  <p className='text-muted-foreground mb-4 text-xs'>
                    {t('Last checked')}{' '}
                    {new Date(latest.checked_at).toLocaleString(
                      toIntlLocale(i18n.language)
                    )}
                  </p>
                  <dl className='bg-muted/20 grid grid-cols-3 gap-3 rounded-xl border p-4'>
                    {[
                      { label: t('Total'), count: results.length },
                      {
                        label: t('Matched'),
                        count: results.filter(
                          (item) => item.verdict === 'match'
                        ).length,
                      },
                      {
                        label: t('Possible mismatch'),
                        count: results.filter(
                          (item) => item.verdict === 'mismatch'
                        ).length,
                      },
                    ].map((item) => (
                      <div key={item.label} className='min-w-0'>
                        <dt className='text-muted-foreground text-xs break-words'>
                          {item.label}
                        </dt>
                        <dd className='mt-1 text-2xl font-semibold tabular-nums'>
                          {formatNumber(item.count, locale)}
                        </dd>
                      </div>
                    ))}
                  </dl>
                  {results.length > 0 && (
                    <div className='flex flex-wrap items-end gap-3'>
                      <div className='min-w-0 flex-1 basis-48 space-y-1.5'>
                        <Label htmlFor='fingerprint-search'>
                          {t('Search')}
                        </Label>
                        <Input
                          id='fingerprint-search'
                          type='search'
                          value={search}
                          onChange={(event) => setSearch(event.target.value)}
                          placeholder={`${t('Model')} / ${t('Group')}`}
                        />
                      </div>
                      <div className='space-y-1.5'>
                        <Label htmlFor='fingerprint-verdict'>
                          {t('Status')}
                        </Label>
                        <NativeSelect
                          id='fingerprint-verdict'
                          value={verdictFilter}
                          onChange={(event) =>
                            setVerdictFilter(event.target.value)
                          }
                        >
                          <NativeSelectOption value='all'>
                            {t('All Status')}
                          </NativeSelectOption>
                          <NativeSelectOption value='match'>
                            {t('Matched')}
                          </NativeSelectOption>
                          <NativeSelectOption value='mismatch'>
                            {t('Possible mismatch')}
                          </NativeSelectOption>
                          <NativeSelectOption value='inconclusive'>
                            {t('Inconclusive')}
                          </NativeSelectOption>
                          <NativeSelectOption value='reference_missing'>
                            {t('Reference not found')}
                          </NativeSelectOption>
                          <NativeSelectOption value='unavailable'>
                            {t('Detection unavailable')}
                          </NativeSelectOption>
                        </NativeSelect>
                      </div>
                      {hasFilters && (
                        <Button
                          variant='ghost'
                          onClick={() => {
                            setSearch('')
                            setVerdictFilter('all')
                          }}
                        >
                          {t('Clear filters')}
                        </Button>
                      )}
                    </div>
                  )}
                  {results.length > 0 && filteredResults.length === 0 && (
                    <EmptyState
                      title={t('No results found')}
                      className='min-h-40'
                    />
                  )}
                  <ul className='grid gap-3 sm:grid-cols-2 xl:grid-cols-3'>
                    {filteredResults.map((result) => (
                      <li key={JSON.stringify([result.group, result.model])}>
                        <div className='bg-muted/20 flex h-full flex-col gap-4 rounded-xl border p-4'>
                          <div className='min-w-0 space-y-1.5'>
                            <p
                              className='text-muted-foreground truncate text-xs'
                              title={result.group}
                            >
                              {result.group}
                            </p>
                            <h3
                              className='truncate text-base font-semibold'
                              title={result.model}
                            >
                              {result.model}
                            </h3>
                            <FingerprintVerdict verdict={result.verdict} />
                          </div>
                          <CandidateConfidence analysis={result.analysis} />
                          <p className='text-muted-foreground text-xs'>
                            {t('Last checked')}{' '}
                            {new Date(
                              result.checked_at || latest.checked_at
                            ).toLocaleString(locale)}
                          </p>
                          <Button
                            size='sm'
                            variant='outline'
                            className='mt-auto self-start'
                            onClick={() => setDetail(result)}
                          >
                            {t('Details')}
                          </Button>
                        </div>
                      </li>
                    ))}
                  </ul>
                </>
              )}
            </CardContent>
          </Card>
          {admin && (
            <Card>
              <CardHeader>
                <CardTitle>{t('Fingerprint targets')}</CardTitle>
              </CardHeader>
              <CardContent className='space-y-4'>
                {config.data?.base_url && (
                  <dl className='bg-muted/30 flex flex-wrap items-baseline gap-x-3 gap-y-1 rounded-lg border px-3 py-2 text-sm'>
                    <dt className='text-muted-foreground'>{t('Base URL')}</dt>
                    <dd className='font-mono text-xs break-all'>
                      {config.data.base_url}
                    </dd>
                  </dl>
                )}
                <p className='text-muted-foreground text-sm'>
                  {t(
                    'Each target sends three requests through one channel and may incur upstream charges.'
                  )}
                </p>
                {config.isPending && <p role='status'>{t('Loading...')}</p>}
                {targets.map((target, index) => (
                  <div
                    key={target.rowId}
                    className='grid items-end gap-3 rounded-lg border p-3 sm:grid-cols-[1fr_1fr_1fr_auto]'
                  >
                    <div className='min-w-0 space-y-1.5'>
                      <Label htmlFor={`fingerprint-group-${index}`}>
                        {t('Group')}
                      </Label>
                      <Combobox
                        id={`fingerprint-group-${index}`}
                        options={groups}
                        value={target.group}
                        disabled={busy || !canWrite}
                        onValueChange={(group) =>
                          setDraft(
                            targets.map((item, i) =>
                              i === index
                                ? {
                                    rowId: item.rowId,
                                    group: group ?? '',
                                    model: '',
                                    reference: '',
                                  }
                                : item
                            )
                          )
                        }
                      />
                    </div>
                    <div className='min-w-0 space-y-1.5'>
                      <Label htmlFor={`fingerprint-model-${index}`}>
                        {t('Advertised model')}
                      </Label>
                      <Combobox
                        id={`fingerprint-model-${index}`}
                        options={(catalog[target.group] ?? []).map((model) => ({
                          value: model,
                          label: model,
                        }))}
                        value={target.model}
                        disabled={busy || !canWrite || !target.group}
                        onValueChange={(model) =>
                          setDraft(
                            targets.map((item, i) =>
                              i === index
                                ? { ...item, model: model ?? '', reference: '' }
                                : item
                            )
                          )
                        }
                      />
                    </div>
                    <div className='min-w-0 space-y-1.5'>
                      <Label htmlFor={`fingerprint-reference-${index}`}>
                        {t('Reference model')}
                      </Label>
                      <Input
                        id={`fingerprint-reference-${index}`}
                        placeholder={target.model}
                        maxLength={255}
                        value={target.reference}
                        disabled={busy || !canWrite}
                        onChange={(event) =>
                          setDraft(
                            targets.map((item, i) =>
                              i === index
                                ? { ...item, reference: event.target.value }
                                : item
                            )
                          )
                        }
                      />
                    </div>
                    <Button
                      aria-label={t('Remove target')}
                      size='icon'
                      variant='ghost'
                      disabled={busy || !canWrite}
                      onClick={() =>
                        setDraft(targets.filter((_, i) => i !== index))
                      }
                    >
                      <X />
                    </Button>
                  </div>
                ))}
                {config.data && (
                  <div className='flex flex-wrap items-center gap-4'>
                    <div className='flex items-center gap-2'>
                      <Switch
                        id='fp-auto'
                        checked={schedule.enabled}
                        disabled={busy || !canWrite}
                        onCheckedChange={(enabled) =>
                          setTimerDraft({ ...schedule, enabled })
                        }
                      />
                      <Label htmlFor='fp-auto'>
                        {t('Scheduled fingerprint checks')}
                      </Label>
                    </div>
                    <div className='flex items-center gap-2'>
                      <Label htmlFor='fp-interval'>
                        {t('Test interval (minutes)')}
                      </Label>
                      <Input
                        id='fp-interval'
                        className='w-24'
                        type='number'
                        min={1}
                        max={1440}
                        step={1}
                        value={schedule.minutes || ''}
                        disabled={!canWrite || busy || !schedule.enabled}
                        onChange={(e) =>
                          setTimerDraft({
                            ...schedule,
                            minutes: Number(e.target.value),
                          })
                        }
                      />
                    </div>
                  </div>
                )}
                {canWrite && config.data && (
                  <div className='flex flex-wrap gap-2'>
                    <Button
                      variant='outline'
                      disabled={
                        busy ||
                        groups.length === 0 ||
                        targets.length >= config.data.max_targets
                      }
                      onClick={() =>
                        setDraft([
                          ...targets,
                          {
                            group: '',
                            model: '',
                            reference: '',
                            rowId: nanoid(),
                          },
                        ])
                      }
                    >
                      <Plus data-icon='inline-start' />
                      {t('Add target')}
                    </Button>
                    <Button
                      disabled={busy || !dirty || !valid}
                      onClick={() =>
                        save.mutate(
                          targets.map(({ group, model, reference }) => ({
                            group,
                            model,
                            reference,
                          }))
                        )
                      }
                    >
                      {t('Save')}
                    </Button>
                    {dirty && (
                      <Button
                        variant='ghost'
                        disabled={busy || !canWrite}
                        onClick={() => {
                          setDraft(null)
                          setTimerDraft(null)
                        }}
                      >
                        {t('Cancel')}
                      </Button>
                    )}
                  </div>
                )}
                {targets.length > 0 && (
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'Set a reference model for aliases. Leave it empty to compare with the advertised model.'
                    )}
                  </p>
                )}
              </CardContent>
            </Card>
          )}
        </div>
        <FingerprintDetails result={detail} onClose={() => setDetail(null)} />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function FingerprintVerdict({ verdict }: { verdict: Verdict }) {
  const { t } = useTranslation()
  switch (verdict) {
    case 'match':
      return <Badge>{t('Matched')}</Badge>
    case 'mismatch':
      return <Badge variant='destructive'>{t('Possible mismatch')}</Badge>
    case 'reference_missing':
      return <Badge variant='outline'>{t('Reference not found')}</Badge>
    case 'inconclusive':
      return <Badge variant='warning'>{t('Inconclusive')}</Badge>
    default:
      return <Badge variant='secondary'>{t('Detection unavailable')}</Badge>
  }
}

function CandidateConfidence(props: { analysis: Result['analysis'] }) {
  const { t } = useTranslation()
  const analysis = props.analysis
  const candidate = analysis?.results?.find(
    (item) => item.model === analysis.prediction
  )
  let name = '—'
  let percent: number | null = null
  if (analysis?.prediction) {
    name =
      candidate?.display_name || analysis.prediction_name || analysis.prediction
    // Keep the displayed candidate and probability paired, including missing values.
    const probability = candidate ? candidate.probability : analysis.probability
    if (analysis.decision === 'scored' && probability != null) {
      percent = probability * 100
    }
  }
  return (
    <div className='min-w-0 space-y-3'>
      <dl className='space-y-1'>
        <dt className='text-muted-foreground text-xs'>
          {t('Closest candidate')}
        </dt>
        <dd className='truncate text-sm font-medium' title={name}>
          {name}
        </dd>
      </dl>
      {percent == null ? (
        <div className='flex items-baseline justify-between gap-2 text-sm'>
          <span className='text-muted-foreground text-xs'>
            {t('Candidate confidence')}
          </span>
          <span>—</span>
        </div>
      ) : (
        <Progress value={percent} className='items-baseline gap-x-2 gap-y-2'>
          <ProgressLabel className='text-muted-foreground text-xs'>
            <span aria-hidden='true'>{t('Candidate confidence')}</span>
            <span className='sr-only'>
              {t('Confidence in {{model}}', { model: name })}
            </span>
          </ProgressLabel>
          <span className='ml-auto text-base font-semibold tabular-nums'>
            {formatPercent(percent)}
          </span>
        </Progress>
      )}
    </div>
  )
}

function FingerprintDetails({
  result,
  onClose,
}: {
  result: Result | null
  onClose: () => void
}) {
  const { t, i18n } = useTranslation()
  const analysis = result?.analysis
  return (
    <Dialog
      open={result !== null}
      onOpenChange={(open) => {
        if (!open) onClose()
      }}
    >
      <DialogContent className='max-h-[85dvh] overflow-auto sm:max-w-3xl'>
        {result && (
          <>
            <DialogHeader>
              <DialogTitle>{result.model}</DialogTitle>
              <DialogDescription>
                {t(
                  'A higher score means a closer match. Candidate confidence is the estimated probability among reference models. A high value for a different candidate indicates a possible mismatch.'
                )}
              </DialogDescription>
            </DialogHeader>
            <div className='bg-muted/30 grid gap-3 rounded-xl border p-4'>
              <div>
                <FingerprintVerdict verdict={result.verdict} />
              </div>
              <CandidateConfidence analysis={analysis} />
            </div>
            <dl className='grid gap-3 text-sm sm:grid-cols-2'>
              <div>
                <dt className='text-muted-foreground'>{t('Group')}</dt>
                <dd>{result.group}</dd>
              </div>
              {result.channel_id && (
                <div>
                  <dt className='text-muted-foreground'>{t('Channel')}</dt>
                  <dd>
                    {result.channel_id
                      ? `#${result.channel_id} ${result.channel_name ?? ''}`
                      : '—'}
                  </dd>
                </div>
              )}
              <div>
                <dt className='text-muted-foreground'>
                  {t('Reference model')}
                </dt>
                <dd>{result.reference || result.model}</dd>
              </div>
              {result.channel_id && (
                <div>
                  <dt className='text-muted-foreground'>
                    {t('Upstream-reported model')}
                  </dt>
                  <dd>{result.reported_model || '—'}</dd>
                </div>
              )}
              <div>
                <dt className='text-muted-foreground'>{t('Valid samples')}</dt>
                <dd>
                  {t('{{count}} of 3 fingerprint samples', {
                    count: analysis?.used_outputs ?? 0,
                  })}
                </dd>
              </div>
            </dl>
            {analysis?.diagnostics && (
              <div className='flex flex-wrap gap-2'>
                {analysis.diagnostics.map((sample) => (
                  <Badge
                    key={sample.index}
                    variant={sample.accepted ? 'outline' : 'warning'}
                  >
                    {t('Sample {{index}}: {{parsed}} / {{minimum}} numbers', {
                      index: sample.index + 1,
                      parsed: sample.parsed_numbers,
                      minimum: sample.minimum_numbers,
                    })}
                  </Badge>
                ))}
              </div>
            )}
            {analysis?.decision === 'partial' && (
              <p className='text-muted-foreground text-sm'>
                {t(
                  'Three valid samples are required for calibrated confidence.'
                )}
              </p>
            )}
            {analysis?.decision === 'not_model' && (
              <p className='text-muted-foreground text-sm'>
                {t('The samples did not exhibit a model-like pattern.')}
              </p>
            )}
            {analysis?.results?.length ? (
              <Table className='min-w-[34rem]'>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('Candidate model')}</TableHead>
                    <TableHead>{t('Model family')}</TableHead>
                    <TableHead className='text-right'>
                      {t('Similarity score')}
                    </TableHead>
                    <TableHead className='text-right'>
                      {t('Candidate confidence')}
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {analysis.results.map((candidate) => (
                    <TableRow key={candidate.model}>
                      <TableCell>
                        <div>{candidate.display_name || candidate.model}</div>
                        {candidate.display_name &&
                          candidate.display_name !== candidate.model && (
                            <div className='text-muted-foreground text-xs'>
                              {candidate.model}
                            </div>
                          )}
                      </TableCell>
                      <TableCell>{candidate.family_name}</TableCell>
                      <TableCell className='text-right tabular-nums'>
                        {formatNumber(
                          candidate.score,
                          toIntlLocale(i18n.language)
                        )}
                      </TableCell>
                      <TableCell className='text-right tabular-nums'>
                        {formatPercent(
                          candidate.probability == null
                            ? null
                            : candidate.probability * 100
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ) : null}
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
