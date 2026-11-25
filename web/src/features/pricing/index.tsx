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
import { lazy, Suspense, useCallback, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { ErrorState } from '@/components/error-state'
import { PublicLayout, PublicPageHero } from '@/components/layout'
import { LoadingState } from '@/components/loading-state'
import { PageTransition } from '@/components/page-transition'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { getServerErrorMessage } from '@/lib/server-error-message'

import {
  LoadingSkeleton,
  EmptyState,
  SearchBar,
  PricingTable,
  PricingSidebar,
  PricingToolbar,
  ModelCardGrid,
} from './components'
import { EXCLUDED_GROUPS, VIEW_MODES } from './constants'
import { useFilters } from './hooks/use-filters'
import { usePricingData } from './hooks/use-pricing-data'

const ModelDetailsDrawer = lazy(() =>
  import('./components/model-details').then((module) => ({
    default: module.ModelDetailsDrawer,
  }))
)

export function Pricing() {
  const { t } = useTranslation()
  const [selectedModelName, setSelectedModelName] = useState<string | null>(
    null
  )

  const {
    models,
    vendors,
    groupRatio,
    groupRegions,
    usableGroup,
    endpointMap,
    autoGroups,
    isLoading,
    isFetching,
    hasData,
    error,
    refetch,
    priceRate,
    usdExchangeRate,
  } = usePricingData()

  const {
    searchInput,
    sortBy,
    vendorFilter,
    groupFilter,
    quotaTypeFilter,
    endpointTypeFilter,
    tagFilter,
    tokenUnit,
    viewMode,
    showRechargePrice,
    setSearchInput,
    setSortBy,
    setVendorFilter,
    setGroupFilter,
    setQuotaTypeFilter,
    setEndpointTypeFilter,
    setTagFilter,
    setTokenUnit,
    setViewMode,
    setShowRechargePrice,
    filteredModels,
    hasActiveFilters,
    activeFilterCount,
    availableTags,
    clearFilters,
    clearSearch,
  } = useFilters(models || [])

  const handleModelClick = useCallback((modelName: string) => {
    setSelectedModelName(modelName)
  }, [])

  const selectedModel = useMemo(
    () =>
      selectedModelName
        ? (models || []).find(
            (model) => model.model_name === selectedModelName
          ) || null
        : null,
    [models, selectedModelName]
  )

  const availableGroups = useMemo(
    () =>
      Object.keys(usableGroup || {}).filter(
        (g) => !EXCLUDED_GROUPS.includes(g)
      ),
    [usableGroup]
  )

  const handleClearAll = useCallback(() => {
    clearFilters()
    clearSearch()
  }, [clearFilters, clearSearch])

  const renderPricingContent = () => {
    if (filteredModels.length === 0) {
      return (
        <EmptyState
          searchQuery={searchInput}
          hasActiveFilters={hasActiveFilters}
          onClearFilters={handleClearAll}
        />
      )
    }

    if (viewMode === VIEW_MODES.CARD) {
      return (
        <ModelCardGrid
          models={filteredModels}
          onModelClick={handleModelClick}
          priceRate={priceRate}
          usdExchangeRate={usdExchangeRate}
          tokenUnit={tokenUnit}
          showRechargePrice={showRechargePrice}
          selectedGroup={groupFilter}
        />
      )
    }

    return (
      <PricingTable
        models={filteredModels}
        priceRate={priceRate}
        usdExchangeRate={usdExchangeRate}
        tokenUnit={tokenUnit}
        showRechargePrice={showRechargePrice}
        selectedGroup={groupFilter}
        onModelClick={handleModelClick}
      />
    )
  }

  if (isLoading) {
    return (
      <PublicLayout showMainContainer={false}>
        <div className='mx-auto w-full max-w-[1800px] px-4 py-6 sm:px-8 xl:px-12'>
          <LoadingSkeleton viewMode={viewMode} />
        </div>
      </PublicLayout>
    )
  }

  if (error && !hasData) {
    return (
      <PublicLayout>
        <ErrorState
          title={t('Failed to load models')}
          description={getServerErrorMessage(error)}
          action={
            <Button
              variant='outline'
              disabled={isFetching}
              onClick={() => void refetch()}
            >
              {t('Retry')}
            </Button>
          }
        />
      </PublicLayout>
    )
  }

  return (
    <PublicLayout showMainContainer={false}>
      <PublicPageHero
        size='lg'
        title={t('Model Square')}
        description={t(
          'Discover curated AI models, compare pricing and capabilities, and choose the right model for every scenario.'
        )}
      >
        <div className='flex min-w-0 flex-col-reverse gap-2'>
          <p className='text-muted-foreground text-xs'>
            {t('This site currently has {{count}} models enabled', {
              count: models?.length || 0,
            })}
          </p>
          <SearchBar
            value={searchInput}
            onChange={setSearchInput}
            onClear={clearSearch}
            placeholder={t('Search model name, provider, endpoint, or tag...')}
          />
        </div>
      </PublicPageHero>

      <PageTransition className='studio-catalog relative mx-auto w-full max-w-[1800px] px-4 py-6 sm:px-8 xl:px-12'>
        {error && (
          <Alert variant='destructive' className='mb-4'>
            <AlertDescription className='flex flex-wrap items-center justify-between gap-3'>
              <span>{getServerErrorMessage(error)}</span>
              <Button
                variant='outline'
                size='sm'
                disabled={isFetching}
                onClick={() => void refetch()}
              >
                {t('Retry')}
              </Button>
            </AlertDescription>
          </Alert>
        )}
        <div className='grid gap-6 xl:grid-cols-[minmax(0,1fr)_15rem]'>
          <PricingSidebar
            quotaTypeFilter={quotaTypeFilter}
            endpointTypeFilter={endpointTypeFilter}
            vendorFilter={vendorFilter}
            groupFilter={groupFilter}
            tagFilter={tagFilter}
            onQuotaTypeChange={setQuotaTypeFilter}
            onEndpointTypeChange={setEndpointTypeFilter}
            onVendorChange={setVendorFilter}
            onGroupChange={setGroupFilter}
            onTagChange={setTagFilter}
            vendors={vendors || []}
            groups={availableGroups}
            groupRatios={groupRatio}
            groupRegions={groupRegions}
            tags={availableTags}
            models={models || []}
            hasActiveFilters={hasActiveFilters}
            onClearFilters={clearFilters}
            className='hover-scrollbar sticky top-20 hidden max-h-[calc(100dvh-6rem)] self-start overflow-y-auto xl:col-start-2 xl:row-start-1 xl:block'
          />

          <div className='min-w-0 space-y-4 xl:col-start-1 xl:row-start-1'>
            <PricingToolbar
              filteredCount={filteredModels.length}
              totalCount={models?.length}
              sortBy={sortBy}
              onSortChange={setSortBy}
              tokenUnit={tokenUnit}
              onTokenUnitChange={setTokenUnit}
              showRechargePrice={showRechargePrice}
              onRechargePriceChange={setShowRechargePrice}
              viewMode={viewMode}
              onViewModeChange={setViewMode}
              quotaTypeFilter={quotaTypeFilter}
              endpointTypeFilter={endpointTypeFilter}
              vendorFilter={vendorFilter}
              groupFilter={groupFilter}
              tagFilter={tagFilter}
              onQuotaTypeChange={setQuotaTypeFilter}
              onEndpointTypeChange={setEndpointTypeFilter}
              onVendorChange={setVendorFilter}
              onGroupChange={setGroupFilter}
              onTagChange={setTagFilter}
              vendors={vendors || []}
              groups={availableGroups}
              groupRatios={groupRatio}
              tags={availableTags}
              models={models || []}
              hasActiveFilters={hasActiveFilters}
              activeFilterCount={activeFilterCount}
              onClearFilters={clearFilters}
            />

            {renderPricingContent()}
          </div>
        </div>

        {selectedModel && (
          <Suspense
            fallback={
              <Dialog
                open
                onOpenChange={(open) => {
                  if (!open) setSelectedModelName(null)
                }}
                title={selectedModel.model_name}
              >
                <LoadingState />
              </Dialog>
            }
          >
            <ModelDetailsDrawer
              open={Boolean(selectedModel)}
              onOpenChange={(open) => {
                if (!open) setSelectedModelName(null)
              }}
              model={selectedModel}
              groupRatio={groupRatio || {}}
              usableGroup={usableGroup || {}}
              endpointMap={
                (endpointMap as Record<
                  string,
                  { path?: string; method?: string }
                >) || {}
              }
              autoGroups={autoGroups || []}
              priceRate={priceRate ?? 1}
              usdExchangeRate={usdExchangeRate ?? 1}
              tokenUnit={tokenUnit}
              showRechargePrice={showRechargePrice}
            />
          </Suspense>
        )}
      </PageTransition>
    </PublicLayout>
  )
}
