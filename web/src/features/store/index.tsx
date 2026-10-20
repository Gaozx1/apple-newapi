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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronLeft, ChevronRight, Search, Store } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { SectionPageLayout } from '@/components/layout'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { getServerErrorMessage } from '@/lib/server-error-message'
import { useAuthStore } from '@/stores/auth-store'

import {
  createAfterSalesTicket,
  createStoreProduct,
  deleteStoreProduct,
  getStoreMeta,
  importStoreCodes,
  listMyStoreOrders,
  listMyStoreProducts,
  listMyStoreSales,
  listStoreProducts,
  listStoreReports,
  purchaseStoreProduct,
  reportStoreGroup,
  reviewStoreReport,
  updateStoreProduct,
  type MarketGroup,
} from './api'
import { GroupMarketPanel } from './components/group-market-panel'
import { StoreAfterSalesDialog } from './components/store-after-sales-dialog'
import { StoreCodeDialog } from './components/store-code-dialog'
import { StoreImportDialog } from './components/store-import-dialog'
import { StoreMyProducts } from './components/store-my-products'
import { StoreProductCard } from './components/store-product-card'
import { StorePublishDialog } from './components/store-publish-dialog'
import { StoreRecords } from './components/store-records'
import { StoreReportDialog } from './components/store-report-dialog'
import { StoreReportReview } from './components/store-report-review'
import { StoreTicketPanel } from './components/store-ticket-panel'
import type { StoreOrder, StoreProduct } from './types'

const STORE_PAGE_SIZE = 12
const ADMIN_ROLE = 10

type StoreTab =
  | 'browse'
  | 'market'
  | 'mine'
  | 'purchases'
  | 'sales'
  | 'afterSales'
  | 'review'
  | 'complaints'

/**
 * 商店:卡密商品走购买流程,用户分组走上架 + 领取流程。领取分组后它的模型才会
 * 出现在买家的模型广场里,令牌也才能调用它。
 */
export function StorePage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const role = useAuthStore((state) => state.auth.user?.role ?? 0)
  const isAdmin = role >= ADMIN_ROLE

  const [tab, setTab] = useState<StoreTab>('browse')
  const [keywordInput, setKeywordInput] = useState('')
  const [keyword, setKeyword] = useState('')
  const [page, setPage] = useState(1)
  const [publishOpen, setPublishOpen] = useState(false)
  const [editing, setEditing] = useState<StoreProduct | null>(null)
  const [importTarget, setImportTarget] = useState<StoreProduct | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<StoreProduct | null>(null)
  const [buyTarget, setBuyTarget] = useState<StoreProduct | null>(null)
  const [boughtOrder, setBoughtOrder] = useState<StoreOrder | null>(null)
  const [reportTarget, setReportTarget] = useState<MarketGroup | null>(null)

  const [afterSalesTarget, setAfterSalesTarget] = useState<StoreOrder | null>(
    null
  )

  const afterSalesMutation = useMutation({
    mutationFn: (payload: { order: StoreOrder; reason: string }) =>
      createAfterSalesTicket(payload.order.id, payload.reason),
    onSuccess: () => {
      toast.success(t('After-sales case opened'))
      setAfterSalesTarget(null)
      void queryClient.invalidateQueries({ queryKey: ['store-tickets'] })
    },
    onError: (error) =>
      toast.error(getServerErrorMessage(error, t('Save failed'))),
  })

  const metaQuery = useQuery({
    queryKey: ['store-meta'],
    queryFn: getStoreMeta,
  })
  const productsQuery = useQuery({
    queryKey: ['store-products', keyword, page],
    queryFn: () => listStoreProducts(keyword, page, STORE_PAGE_SIZE),
  })
  const mineQuery = useQuery({
    queryKey: ['store-mine'],
    queryFn: () => listMyStoreProducts(1, 100),
    enabled: tab === 'mine',
  })
  const ordersQuery = useQuery({
    queryKey: ['store-orders'],
    queryFn: () => listMyStoreOrders(1, 100),
    enabled: tab === 'purchases',
  })
  const salesQuery = useQuery({
    queryKey: ['store-sales'],
    queryFn: () => listMyStoreSales(1, 100),
    enabled: tab === 'sales',
  })
  const reportsQuery = useQuery({
    queryKey: ['store-reports'],
    queryFn: () => listStoreReports(0, 1, 100),
    enabled: tab === 'review' && isAdmin,
  })

  const invalidateStore = () => {
    for (const key of [
      'store-products',
      'store-mine',
      'store-orders',
      'store-sales',
      'store-reports',
      'store-market-groups',
    ]) {
      void queryClient.invalidateQueries({ queryKey: [key] })
    }
  }

  const fail = (fallback: string) => (error: unknown) =>
    toast.error(getServerErrorMessage(error, t(fallback)))

  const saveMutation = useMutation({
    mutationFn: async (values: {
      name: string
      description: string
      price: string
    }) => {
      const input = {
        name: values.name,
        description: values.description,
        price_amount: Number(values.price),
      }
      if (editing === null) return createStoreProduct(input)
      return updateStoreProduct(editing.id, {
        ...input,
        status: editing.status,
      })
    },
    onSuccess: () => {
      toast.success(t('Saved'))
      setPublishOpen(false)
      setEditing(null)
      invalidateStore()
    },
    onError: fail('Save failed'),
  })

  const statusMutation = useMutation({
    mutationFn: (product: StoreProduct) =>
      updateStoreProduct(product.id, {
        name: product.name,
        description: product.description,
        price_amount: product.price_amount,
        status: product.status === 1 ? 0 : 1,
      }),
    onSuccess: () => {
      toast.success(t('Saved'))
      invalidateStore()
    },
    onError: fail('Save failed'),
  })

  const deleteMutation = useMutation({
    mutationFn: (product: StoreProduct) => deleteStoreProduct(product.id),
    onSuccess: () => {
      toast.success(t('Deleted'))
      setDeleteTarget(null)
      invalidateStore()
    },
    onError: fail('Delete failed'),
  })

  const importMutation = useMutation({
    mutationFn: (codes: string) => {
      if (importTarget === null) throw new Error(t('No product selected'))
      return importStoreCodes(importTarget.id, codes)
    },
    onSuccess: (result) => {
      toast.success(
        t('Imported {{imported}} codes, skipped {{duplicated}} duplicates', {
          imported: result.imported,
          duplicated: result.duplicated,
        })
      )
      setImportTarget(null)
      invalidateStore()
    },
    onError: fail('Import failed'),
  })

  const buyMutation = useMutation({
    mutationFn: (product: StoreProduct) => purchaseStoreProduct(product.id),
    onSuccess: (order) => {
      setBoughtOrder(order)
      setBuyTarget(null)
      invalidateStore()
    },
    onError: fail('Purchase failed'),
  })

  const reportMutation = useMutation({
    mutationFn: (payload: { groupId: number; reason: string }) =>
      reportStoreGroup(payload.groupId, payload.reason),
    onSuccess: () => {
      toast.success(t('Report submitted, an administrator will review it'))
      setReportTarget(null)
      invalidateStore()
    },
    onError: fail('Report failed'),
  })

  const reviewMutation = useMutation({
    mutationFn: (payload: {
      reportId: number
      accept: boolean
      note: string
    }) => reviewStoreReport(payload.reportId, payload.accept, payload.note),
    onSuccess: () => {
      toast.success(t('Report updated'))
      invalidateStore()
    },
    onError: fail('Save failed'),
  })

  const products = productsQuery.data?.items ?? []
  const total = productsQuery.data?.total ?? 0
  const pageCount = Math.max(1, Math.ceil(total / STORE_PAGE_SIZE))
  const feePermille = metaQuery.data?.permille ?? 1

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Store')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='grid gap-4'>
          <Card>
            <CardHeader>
              <CardTitle className='flex items-center gap-2'>
                <Store className='size-4' aria-hidden='true' />
                {t('Store')}
              </CardTitle>
              <CardDescription>
                {t(
                  'Buy a code, or claim a user group. The platform keeps {{rate}} of every code sale and 1% of every channel sale.',
                  { rate: `${feePermille / 10}%` }
                )}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Tabs
                value={tab}
                onValueChange={(value) => setTab(value as StoreTab)}
              >
                <TabsList className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'>
                  <TabsTrigger value='browse'>{t('All products')}</TabsTrigger>
                  <TabsTrigger value='market'>{t('Group market')}</TabsTrigger>
                  <TabsTrigger value='mine'>{t('My products')}</TabsTrigger>
                  <TabsTrigger value='purchases'>
                    {t('My purchases')}
                  </TabsTrigger>
                  <TabsTrigger value='sales'>{t('My sales')}</TabsTrigger>
                  <TabsTrigger value='afterSales'>
                    {t('After-sales')}
                  </TabsTrigger>
                  {isAdmin && (
                    <TabsTrigger value='review'>
                      {t('Report review')}
                    </TabsTrigger>
                  )}
                </TabsList>

                <TabsContent value='browse' className='grid gap-3'>
                  <form
                    className='flex gap-2'
                    onSubmit={(event) => {
                      event.preventDefault()
                      setPage(1)
                      setKeyword(keywordInput.trim())
                    }}
                  >
                    <Input
                      value={keywordInput}
                      onChange={(event) => setKeywordInput(event.target.value)}
                      placeholder={t('Search products')}
                      aria-label={t('Search products')}
                    />
                    <Button type='submit' variant='outline' size='icon'>
                      <Search className='size-4' aria-hidden='true' />
                    </Button>
                  </form>

                  {productsQuery.isLoading && <LoadingState />}
                  {!productsQuery.isLoading && products.length === 0 && (
                    <p className='text-muted-foreground text-sm'>
                      {t('No products yet')}
                    </p>
                  )}
                  <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-3'>
                    {products.map((product) => (
                      <StoreProductCard
                        key={product.id}
                        product={product}
                        buying={buyMutation.isPending}
                        onBuy={(target) => setBuyTarget(target)}
                      />
                    ))}
                  </div>
                </TabsContent>

                <TabsContent value='market'>
                  <GroupMarketPanel
                    onReport={(group) => setReportTarget(group)}
                  />
                </TabsContent>

                <TabsContent value='mine'>
                  <StoreMyProducts
                    products={mineQuery.data?.items ?? []}
                    onPublish={() => {
                      setEditing(null)
                      setPublishOpen(true)
                    }}
                    onEdit={(product) => {
                      setEditing(product)
                      setPublishOpen(true)
                    }}
                    onImport={(product) => setImportTarget(product)}
                    onToggleStatus={(product) => statusMutation.mutate(product)}
                    onDelete={(product) => setDeleteTarget(product)}
                  />
                </TabsContent>

                <TabsContent value='purchases'>
                  <StoreRecords
                    mode='purchases'
                    orders={ordersQuery.data?.items ?? []}
                    onAfterSales={setAfterSalesTarget}
                  />
                </TabsContent>

                <TabsContent value='sales'>
                  <StoreRecords
                    mode='sales'
                    orders={salesQuery.data?.items ?? []}
                  />
                </TabsContent>

                {isAdmin && (
                  <TabsContent value='review'>
                    <StoreReportReview
                      reports={reportsQuery.data?.items ?? []}
                      onReview={(report, accept, note) =>
                        reviewMutation.mutate({
                          reportId: report.id,
                          accept,
                          note,
                        })
                      }
                    />
                  </TabsContent>
                )}
                {isAdmin && (
                  <TabsContent value='complaints'>
                    <StoreTicketPanel mode='admin' />
                  </TabsContent>
                )}
              </Tabs>

              {pageCount > 1 && tab === 'browse' && (
                <div className='mt-3 flex items-center justify-end gap-2'>
                  <Button
                    size='icon'
                    variant='outline'
                    disabled={page <= 1}
                    aria-label={t('Previous page')}
                    onClick={() =>
                      setPage((current) => Math.max(1, current - 1))
                    }
                  >
                    <ChevronLeft className='size-4' aria-hidden='true' />
                  </Button>
                  <span className='text-muted-foreground text-xs'>
                    {page} / {pageCount}
                  </span>
                  <Button
                    size='icon'
                    variant='outline'
                    disabled={page >= pageCount}
                    aria-label={t('Next page')}
                    onClick={() =>
                      setPage((current) => Math.min(pageCount, current + 1))
                    }
                  >
                    <ChevronRight className='size-4' aria-hidden='true' />
                  </Button>
                </div>
              )}
            </CardContent>
          </Card>

          <StorePublishDialog
            open={publishOpen}
            saving={saveMutation.isPending}
            product={editing}
            feePermille={feePermille}
            onOpenChange={(open) => {
              setPublishOpen(open)
              if (!open) setEditing(null)
            }}
            onSubmit={(values) => saveMutation.mutate(values)}
          />

          <StoreAfterSalesDialog
            open={afterSalesTarget !== null}
            saving={afterSalesMutation.isPending}
            order={afterSalesTarget}
            onOpenChange={(open) => {
              if (!open) setAfterSalesTarget(null)
            }}
            onSubmit={(reason) => {
              if (afterSalesTarget !== null) {
                afterSalesMutation.mutate({ order: afterSalesTarget, reason })
              }
            }}
          />
          <StoreImportDialog
            open={importTarget !== null}
            saving={importMutation.isPending}
            product={importTarget}
            maxCodes={metaQuery.data?.max_codes_per_import ?? 500}
            onOpenChange={(open) => {
              if (!open) setImportTarget(null)
            }}
            onSubmit={(codes) => importMutation.mutate(codes)}
          />

          <StoreCodeDialog
            order={boughtOrder}
            onOpenChange={(open) => {
              if (!open) setBoughtOrder(null)
            }}
          />

          <StoreReportDialog
            open={reportTarget !== null}
            saving={reportMutation.isPending}
            name={reportTarget?.public_name ?? ''}
            onOpenChange={(open) => {
              if (!open) setReportTarget(null)
            }}
            onSubmit={(reason) => {
              if (reportTarget !== null) {
                reportMutation.mutate({ groupId: reportTarget.id, reason })
              }
            }}
          />

          {/* Buying a code product always requires an explicit confirmation. */}
          <ConfirmDialog
            open={buyTarget !== null}
            title={t('Confirm purchase')}
            desc={
              buyTarget === null
                ? ''
                : t(
                    'Pay {{price}} for {{name}}? The code is shown right after.',
                    {
                      price: formatCurrencyFromUSD(buyTarget.price_amount),
                      name: buyTarget.name,
                    }
                  )
            }
            confirmText={t('Confirm and pay')}
            isLoading={buyMutation.isPending}
            handleConfirm={() => {
              if (buyTarget !== null) buyMutation.mutate(buyTarget)
            }}
            onOpenChange={(open) => {
              if (!open) setBuyTarget(null)
            }}
          />

          <ConfirmDialog
            open={deleteTarget !== null}
            title={t('Delete product')}
            desc={t(
              'Unsold codes are removed with the product. Delivered codes stay readable for their buyers.'
            )}
            destructive
            isLoading={deleteMutation.isPending}
            handleConfirm={() => {
              if (deleteTarget !== null) deleteMutation.mutate(deleteTarget)
            }}
            onOpenChange={(open) => {
              if (!open) setDeleteTarget(null)
            }}
          />
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
