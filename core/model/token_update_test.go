package model_test

import (
	"path/filepath"
	"testing"

	"github.com/labring/aiproxy/core/model"
)

func TestUpdateTokenRejectsInvalidScope(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "tokens.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	prevDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = prevDB
	})

	if err := db.AutoMigrate(&model.Token{}); err != nil {
		t.Fatalf("migrate tokens: %v", err)
	}

	token := model.Token{
		GroupID: "test-group",
		Name:    model.EmptyNullString("test-token"),
		Scope:   model.ChannelScopeGlobal,
		Status:  model.TokenStatusEnabled,
	}
	if err := db.Create(&token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	invalidScope := "glboal"
	if _, err := model.UpdateToken(token.ID, model.UpdateTokenRequest{
		Scope: &invalidScope,
	}); err == nil {
		t.Fatal("expected invalid token scope error")
	}

	var got model.Token
	if err := db.First(&got, token.ID).Error; err != nil {
		t.Fatalf("reload token: %v", err)
	}

	if got.Scope != model.ChannelScopeGlobal {
		t.Fatalf("expected scope to remain global, got %q", got.Scope)
	}
}

func TestUpdateTokenUpdatesGroupChannelModels(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "tokens.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	prevDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = prevDB
	})

	if err := db.AutoMigrate(&model.Token{}); err != nil {
		t.Fatalf("migrate tokens: %v", err)
	}

	token := model.Token{
		GroupID: "test-group",
		Name:    model.EmptyNullString("test-token"),
		Models:  []string{"global-model"},
		Status:  model.TokenStatusEnabled,
	}
	if err := db.Create(&token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	groupChannelModels := []string{"group-model"}
	if _, err := model.UpdateToken(token.ID, model.UpdateTokenRequest{
		GroupChannelModels: &groupChannelModels,
	}); err != nil {
		t.Fatalf("update token: %v", err)
	}

	var got model.Token
	if err := db.First(&got, token.ID).Error; err != nil {
		t.Fatalf("reload token: %v", err)
	}

	if len(got.GroupChannelModels) != 1 || got.GroupChannelModels[0] != "group-model" {
		t.Fatalf("expected group channel models to update, got %v", got.GroupChannelModels)
	}
}

func TestGetTokenByKeyForAuthSkipsQuotaCheck(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "tokens.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	prevDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = prevDB
	})

	if err := db.AutoMigrate(&model.Token{}); err != nil {
		t.Fatalf("migrate tokens: %v", err)
	}

	token := model.Token{
		GroupID:    "test-group",
		Name:       model.EmptyNullString("test-token"),
		Scope:      model.ChannelScopeGroup,
		Quota:      1,
		UsedAmount: 1,
		Status:     model.TokenStatusEnabled,
	}
	if err := db.Create(&token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	got, err := model.GetTokenByKeyForAuth(token.Key)
	if err != nil {
		t.Fatalf("get token for auth: %v", err)
	}

	if got.ID != token.ID {
		t.Fatalf("expected token id %d, got %d", token.ID, got.ID)
	}

	if err := model.ValidateTokenQuota(got); err == nil {
		t.Fatal("expected quota validation error")
	}
}

func TestUpdateGroupTokenRejectsInvalidScope(t *testing.T) {
	db, err := model.OpenSQLite(filepath.Join(t.TempDir(), "group_tokens.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	prevDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = prevDB
	})

	if err := db.AutoMigrate(&model.Token{}); err != nil {
		t.Fatalf("migrate tokens: %v", err)
	}

	token := model.Token{
		GroupID: "test-group",
		Name:    model.EmptyNullString("test-token"),
		Scope:   model.ChannelScopeGlobal,
		Status:  model.TokenStatusEnabled,
	}
	if err := db.Create(&token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}

	invalidScope := "glboal"
	if _, err := model.UpdateGroupToken(token.ID, token.GroupID, model.UpdateTokenRequest{
		Scope: &invalidScope,
	}); err == nil {
		t.Fatal("expected invalid token scope error")
	}

	var got model.Token
	if err := db.First(&got, token.ID).Error; err != nil {
		t.Fatalf("reload token: %v", err)
	}

	if got.Scope != model.ChannelScopeGlobal {
		t.Fatalf("expected scope to remain global, got %q", got.Scope)
	}
}
