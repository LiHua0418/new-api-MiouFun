package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestCalculateAffRechargeRebateQuota(t *testing.T) {
	require.Equal(t, 50, CalculateAffRechargeRebateQuota(1000))
	require.Equal(t, 0, CalculateAffRechargeRebateQuota(19))
	require.Equal(t, 0, CalculateAffRechargeRebateQuota(0))
}

func TestRedeemGrantsAffRechargeRebate(t *testing.T) {
	truncateTables(t)

	inviter := &User{
		Id:       201,
		Username: "aff_inviter",
		Status:   common.UserStatusEnabled,
		AffCode:  "A201",
	}
	invitee := &User{
		Id:        202,
		Username:  "aff_invitee",
		Status:    common.UserStatusEnabled,
		AffCode:   "A202",
		InviterId: inviter.Id,
	}
	require.NoError(t, DB.Create(inviter).Error)
	require.NoError(t, DB.Create(invitee).Error)

	redemption := &Redemption{
		Key:    "12345678901234567890123456789012",
		Status: common.RedemptionCodeStatusEnabled,
		Name:   "aff-redemption",
		Quota:  1000,
	}
	require.NoError(t, DB.Create(redemption).Error)

	quota, err := Redeem(redemption.Key, invitee.Id)
	require.NoError(t, err)
	require.Equal(t, 1000, quota)

	var updatedInviter User
	require.NoError(t, DB.First(&updatedInviter, inviter.Id).Error)
	require.Equal(t, 50, updatedInviter.AffQuota)
	require.Equal(t, 50, updatedInviter.AffHistoryQuota)

	var updatedInvitee User
	require.NoError(t, DB.First(&updatedInvitee, invitee.Id).Error)
	require.Equal(t, 1000, updatedInvitee.Quota)
}
