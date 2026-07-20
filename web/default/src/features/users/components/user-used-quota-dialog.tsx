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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import {
  formatQuota,
  parseQuotaFromDollars,
  quotaUnitsToDollars,
} from '@/lib/format'

import { setUserUsedQuota } from '../api'

interface UserUsedQuotaDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  userId: number
  currentUsedQuota: number
  onSuccess: () => void
}

export function UserUsedQuotaDialog(props: UserUsedQuotaDialogProps) {
  const { t } = useTranslation()
  const [amount, setAmount] = useState('')
  const [loading, setLoading] = useState(false)

  const { meta: currencyMeta } = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()
  const tokensOnly = currencyMeta.kind === 'tokens'

  useEffect(() => {
    if (props.open) {
      setAmount(String(quotaUnitsToDollars(props.currentUsedQuota)))
    }
  }, [props.open, props.currentUsedQuota])

  const parsedAmount = Number(amount)
  const usedQuota = parseQuotaFromDollars(parsedAmount)
  const isValid =
    amount.trim() !== '' &&
    Number.isFinite(parsedAmount) &&
    parsedAmount >= 0 &&
    Number.isSafeInteger(usedQuota)

  const handleConfirm = async () => {
    if (!isValid) return

    setLoading(true)
    try {
      const result = await setUserUsedQuota({
        id: props.userId,
        action: 'set_used_quota',
        value: usedQuota,
      })
      if (result.success) {
        toast.success(t('User updated successfully'))
        props.onOpenChange(false)
        props.onSuccess()
      } else {
        toast.error(result.message || t('Failed to update user'))
      }
    } catch (error: unknown) {
      toast.error(
        error instanceof Error ? error.message : t('Failed to update user')
      )
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Historical Usage')}
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleConfirm} disabled={loading || !isValid}>
            {loading ? t('Processing...') : t('Confirm')}
          </Button>
        </>
      }
    >
      <div className='text-muted-foreground text-sm'>
        {formatQuota(props.currentUsedQuota)}
      </div>
      <div className='space-y-2'>
        <Label>
          {t('Amount')} ({currencyLabel})
        </Label>
        <Input
          type='number'
          step={tokensOnly ? 1 : 0.000001}
          min={0}
          value={amount}
          onChange={(event) => setAmount(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') handleConfirm()
          }}
        />
      </div>
    </Dialog>
  )
}
