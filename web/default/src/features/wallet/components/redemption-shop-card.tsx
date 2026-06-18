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
import { ExternalLink, Gift } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { TitledCard } from '@/components/ui/titled-card'

interface RedemptionShopCardProps {
  shopUrl: string
}

export function RedemptionShopCard({ shopUrl }: RedemptionShopCardProps) {
  const { t } = useTranslation()

  if (!shopUrl) {
    return null
  }

  return (
    <TitledCard
      title={t('Get one here')}
      icon={<Gift className='h-4 w-4' />}
      action={
        <Button
          asChild
          size='sm'
          className='w-full gap-2 whitespace-nowrap sm:w-auto'
        >
          <a
            href={shopUrl}
            target='_blank'
            rel='noopener noreferrer'
            className='inline-flex items-center gap-2 whitespace-nowrap'
          >
            {t('Open in new window')}
            <ExternalLink className='h-4 w-4 shrink-0' />
          </a>
        </Button>
      }
      contentClassName='p-0'
    >
      <iframe
        src={shopUrl}
        title={t('Get one here')}
        className='block h-[680px] w-full border-0 sm:h-[760px]'
        loading='lazy'
        referrerPolicy='no-referrer-when-downgrade'
      />
    </TitledCard>
  )
}
