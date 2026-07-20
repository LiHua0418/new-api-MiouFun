package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupManageUserTest(t *testing.T) *model.User {
	t.Helper()
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}))

	user := &model.User{
		Username:     "managed-user",
		Password:     "password",
		Role:         common.RoleCommonUser,
		Status:       common.UserStatusEnabled,
		Quota:        1000,
		UsedQuota:    20,
		RequestCount: 3,
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

func TestManageUserSetUsedQuota(t *testing.T) {
	user := setupManageUserTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/manage", map[string]interface{}{
		"id":     user.Id,
		"action": "set_used_quota",
		"value":  750,
	}, 99)
	ctx.Set("role", common.RoleAdminUser)
	ctx.Set("username", "admin")

	ManageUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var got model.User
	require.NoError(t, model.DB.First(&got, user.Id).Error)
	assert.Equal(t, 1000, got.Quota)
	assert.Equal(t, 750, got.UsedQuota)
	assert.Equal(t, 3, got.RequestCount)
}

func TestManageUserRejectsNegativeUsedQuota(t *testing.T) {
	user := setupManageUserTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/manage", map[string]interface{}{
		"id":     user.Id,
		"action": "set_used_quota",
		"value":  -1,
	}, 99)
	ctx.Set("role", common.RoleAdminUser)

	ManageUser(ctx)

	response := decodeAPIResponse(t, recorder)
	assert.False(t, response.Success)

	var got model.User
	require.NoError(t, model.DB.First(&got, user.Id).Error)
	assert.Equal(t, 20, got.UsedQuota)
}

func TestManageUserAdjustUsedQuotaModes(t *testing.T) {
	tests := []struct {
		name        string
		mode        string
		value       int
		wantSuccess bool
		wantQuota   int
	}{
		{name: "add", mode: "add", value: 30, wantSuccess: true, wantQuota: 50},
		{name: "subtract", mode: "subtract", value: 15, wantSuccess: true, wantQuota: 5},
		{name: "override zero", mode: "override", value: 0, wantSuccess: true, wantQuota: 0},
		{name: "subtract too much", mode: "subtract", value: 21, wantSuccess: false, wantQuota: 20},
		{name: "reject zero add", mode: "add", value: 0, wantSuccess: false, wantQuota: 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := setupManageUserTest(t)
			ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/manage", map[string]interface{}{
				"id":     user.Id,
				"action": "set_used_quota",
				"mode":   tt.mode,
				"value":  tt.value,
			}, 99)
			ctx.Set("role", common.RoleAdminUser)

			ManageUser(ctx)

			response := decodeAPIResponse(t, recorder)
			assert.Equal(t, tt.wantSuccess, response.Success, response.Message)

			var got model.User
			require.NoError(t, model.DB.First(&got, user.Id).Error)
			assert.Equal(t, tt.wantQuota, got.UsedQuota)
		})
	}
}
