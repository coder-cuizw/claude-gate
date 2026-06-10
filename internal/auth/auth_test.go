package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/claude-gate/claude-gate/internal/domain"
)

func TestGenerateAndParse(t *testing.T) {
	k, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	prefix, secret, err := ParseAPIKey(k.Plaintext)
	if err != nil {
		t.Fatalf("解析生成的 Key 失败: %v", err)
	}
	if prefix != k.Prefix {
		t.Fatalf("前缀不一致: %q vs %q", prefix, k.Prefix)
	}
	if !VerifySecret(secret, k.Hash) {
		t.Fatal("密文校验应通过")
	}
}

func TestParseStripsBearer(t *testing.T) {
	k, _ := GenerateKey()
	_, _, err := ParseAPIKey("Bearer " + k.Plaintext)
	if err != nil {
		t.Fatalf("应能解析带 Bearer 前缀的 Key: %v", err)
	}
}

func TestParseInvalid(t *testing.T) {
	bad := []string{"", "abc", "cg-short-xxx", "xx-AB12cd34-secret", "cg-AB12cd34-"}
	for _, s := range bad {
		if _, _, err := ParseAPIKey(s); err == nil {
			t.Fatalf("非法 Key %q 应解析失败", s)
		}
	}
}

func TestVerifyWrongSecret(t *testing.T) {
	k, _ := GenerateKey()
	if VerifySecret("wrong", k.Hash) {
		t.Fatal("错误密文不应通过校验")
	}
}

// fakeStore 用于解析器测试。
type fakeStore struct {
	rec     *APIKeyRecord
	group   *domain.Group
	channel *domain.UpstreamChannel
	lookErr error
}

func (f *fakeStore) LookupAPIKeyByPrefix(_ context.Context, _ string) (*APIKeyRecord, error) {
	return f.rec, f.lookErr
}
func (f *fakeStore) LoadGroup(_ context.Context, _ int64) (*domain.Group, *domain.UpstreamChannel, error) {
	return f.group, f.channel, nil
}

func mkKey(t *testing.T) (plaintext string, rec *APIKeyRecord) {
	t.Helper()
	k, _ := GenerateKey()
	return k.Plaintext, &APIKeyRecord{ID: 1, GroupID: 10, KeyHash: k.Hash, Enabled: true}
}

func TestResolveSuccess(t *testing.T) {
	pt, rec := mkKey(t)
	store := &fakeStore{
		rec:     rec,
		group:   &domain.Group{ID: 10, Enabled: true, CacheStrategy: domain.CacheStrategyConfig{Type: "passthrough"}},
		channel: &domain.UpstreamChannel{ID: 5, Type: domain.ChannelOfficial},
	}
	rg, err := NewResolver(store).Resolve(context.Background(), pt)
	if err != nil {
		t.Fatal(err)
	}
	if rg.APIKeyID != 1 || rg.Group.ID != 10 || rg.CacheStrategy.Name() != "passthrough" {
		t.Fatalf("解析结果错误: %+v", rg)
	}
}

func TestResolveDistinctErrors(t *testing.T) {
	pt, rec := mkKey(t)
	expired := time.Now().Add(-time.Hour)

	cases := []struct {
		name    string
		store   *fakeStore
		wantErr *domain.Error
	}{
		{"key 不存在", &fakeStore{rec: nil}, domain.ErrInvalidAPIKey},
		{"key 禁用", &fakeStore{rec: &APIKeyRecord{ID: 1, GroupID: 10, KeyHash: rec.KeyHash, Enabled: false}}, domain.ErrAPIKeyDisabled},
		{"key 过期", &fakeStore{rec: &APIKeyRecord{ID: 1, GroupID: 10, KeyHash: rec.KeyHash, Enabled: true, ExpiresAt: &expired}}, domain.ErrAPIKeyExpired},
		{"group 禁用", &fakeStore{rec: rec, group: &domain.Group{ID: 10, Enabled: false}}, domain.ErrGroupDisabled},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewResolver(c.store).Resolve(context.Background(), pt)
			var de *domain.Error
			if !errors.As(err, &de) || de.Code != c.wantErr.Code {
				t.Fatalf("期望错误 %q, 得到 %v", c.wantErr.Code, err)
			}
		})
	}
}

func TestResolveLookupError(t *testing.T) {
	pt, _ := mkKey(t)
	store := &fakeStore{lookErr: errors.New("db down")}
	_, err := NewResolver(store).Resolve(context.Background(), pt)
	var de *domain.Error
	if !errors.As(err, &de) || de.Code != domain.ErrInvalidAPIKey.Code {
		t.Fatalf("查询错误应映射为 invalid_api_key: %v", err)
	}
}
