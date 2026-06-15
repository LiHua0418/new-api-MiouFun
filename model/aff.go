package model

import (
	"fmt"

	"github.com/QuantumNous/new-api/logger"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const AffRechargeRebatePercent = 5

type AffRechargeRebate struct {
	InviterId     int
	InviteeId     int
	RechargeQuota int
	RebateQuota   int
}

func (rebate AffRechargeRebate) Valid() bool {
	return rebate.InviterId > 0 && rebate.InviteeId > 0 && rebate.RebateQuota > 0
}

func CalculateAffRechargeRebateQuota(rechargeQuota int) int {
	if rechargeQuota <= 0 {
		return 0
	}
	return int(decimal.NewFromInt(int64(rechargeQuota)).
		Mul(decimal.NewFromInt(AffRechargeRebatePercent)).
		Div(decimal.NewFromInt(100)).
		IntPart())
}

func GrantAffRechargeRebate(userId int, rechargeQuota int) (AffRechargeRebate, error) {
	return GrantAffRechargeRebateWithTx(DB, userId, rechargeQuota)
}

func GrantAffRechargeRebateWithTx(tx *gorm.DB, userId int, rechargeQuota int) (AffRechargeRebate, error) {
	rebateQuota := CalculateAffRechargeRebateQuota(rechargeQuota)
	if userId == 0 || rebateQuota <= 0 {
		return AffRechargeRebate{}, nil
	}

	var user User
	if err := tx.Model(&User{}).Select("id", "inviter_id").Where("id = ?", userId).First(&user).Error; err != nil {
		return AffRechargeRebate{}, err
	}
	if user.InviterId == 0 || user.InviterId == user.Id {
		return AffRechargeRebate{}, nil
	}

	result := tx.Model(&User{}).Where("id = ?", user.InviterId).Updates(map[string]interface{}{
		"aff_quota":   gorm.Expr("aff_quota + ?", rebateQuota),
		"aff_history": gorm.Expr("aff_history + ?", rebateQuota),
	})
	if result.Error != nil {
		return AffRechargeRebate{}, result.Error
	}
	if result.RowsAffected == 0 {
		return AffRechargeRebate{}, nil
	}

	return AffRechargeRebate{
		InviterId:     user.InviterId,
		InviteeId:     user.Id,
		RechargeQuota: rechargeQuota,
		RebateQuota:   rebateQuota,
	}, nil
}

func RecordAffRechargeRebateLog(rebate AffRechargeRebate, source string) {
	if !rebate.Valid() {
		return
	}
	RecordLog(rebate.InviterId, LogTypeSystem, fmt.Sprintf(
		"邀请用户 %d 通过%s充值返利 %s，充值额度 %s",
		rebate.InviteeId,
		source,
		logger.LogQuota(rebate.RebateQuota),
		logger.LogQuota(rebate.RechargeQuota),
	))
}
