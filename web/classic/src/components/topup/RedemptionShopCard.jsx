/*
Copyright (C) 2025 QuantumNous

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

import React from 'react';
import { Button, Card, Typography } from '@douyinfe/semi-ui';
import { IconGift } from '@douyinfe/semi-icons';

const { Text } = Typography;

const RedemptionShopCard = ({ t, shopUrl }) => {
  if (!shopUrl) {
    return null;
  }

  return (
    <Card
      className='!rounded-2xl shadow-sm border-0'
      title={
        <div className='flex items-center gap-2'>
          <IconGift />
          <Text strong>{t('购买兑换码')}</Text>
        </div>
      }
      headerExtraContent={
        <Button
          theme='solid'
          type='primary'
          size='small'
          onClick={() => window.open(shopUrl, '_blank')}
        >
          {t('新窗口打开')}
        </Button>
      }
    >
      <div className='overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-900'>
        <iframe
          src={shopUrl}
          title={t('购买兑换码')}
          className='block h-[760px] w-full border-0 max-sm:h-[680px]'
          loading='lazy'
          referrerPolicy='no-referrer-when-downgrade'
        />
      </div>
    </Card>
  );
};

export default RedemptionShopCard;
