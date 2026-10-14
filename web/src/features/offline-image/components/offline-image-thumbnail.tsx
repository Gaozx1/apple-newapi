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
import { useQuery } from '@tanstack/react-query'
import { ImageOff } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { api } from '@/lib/api'

import { offlineImageReferenceUrl, offlineImageUrl } from '../api'

function extensionFor(mime: string): string {
  if (mime === 'image/jpeg') return 'jpg'
  const [, subtype] = mime.split('/')
  return subtype || 'png'
}

/**
 * One stored image of a task. The dashboard authenticates with a bearer token,
 * which a plain <img> request cannot carry, so the bytes are fetched through the
 * shared client and rendered from a local object URL.
 */
export function OfflineImageThumbnail(props: {
  taskId: number
  /** Output index; omit to show the task's reference image instead. */
  index?: number
}) {
  const { t } = useTranslation()
  const isReference = props.index === undefined
  const url = isReference
    ? offlineImageReferenceUrl(props.taskId)
    : offlineImageUrl(props.taskId, props.index as number)

  const query = useQuery({
    queryKey: ['offline-image-file', props.taskId, props.index ?? 'reference'],
    queryFn: async ({ signal }) => {
      const response = await api.get<Blob>(url, {
        responseType: 'blob',
        signal,
        // React Query owns deduplication and cancellation for this key.
        disableDuplicate: true,
        skipErrorHandler: true,
        skipBusinessError: true,
      })
      return response.data.type.startsWith('image/') ? response.data : null
    },
    staleTime: 5 * 60 * 1000,
    retry: false,
    meta: { errorToast: false },
  })

  const [src, setSrc] = useState('')
  useEffect(() => {
    if (!query.data) return
    const objectUrl = URL.createObjectURL(query.data)
    setSrc(objectUrl)
    return () => URL.revokeObjectURL(objectUrl)
  }, [query.data])

  if (!query.data || !src) {
    return (
      <div
        className='bg-muted text-muted-foreground flex h-24 w-24 items-center justify-center rounded-md border'
        role='img'
        aria-label={
          isReference ? t('Reference image') : t('Generated image is loading')
        }
      >
        {query.isError ? (
          <ImageOff className='size-4' aria-hidden='true' />
        ) : (
          <span className='sr-only'>{t('Loading')}</span>
        )}
      </div>
    )
  }

  const fileName = isReference
    ? `reference.${extensionFor(query.data.type)}`
    : `image-${(props.index as number) + 1}.${extensionFor(query.data.type)}`

  return (
    <a
      href={src}
      download={fileName}
      aria-label={
        isReference
          ? t('Download the reference image')
          : t('Download generated image {{index}}', {
              index: (props.index as number) + 1,
            })
      }
      className='focus-visible:ring-ring rounded-md focus-visible:ring-2'
    >
      <img
        src={src}
        alt={
          isReference
            ? t('Reference image')
            : t('Generated image {{index}}', {
                index: (props.index as number) + 1,
              })
        }
        className='bg-muted h-24 w-24 rounded-md border object-cover'
      />
    </a>
  )
}
