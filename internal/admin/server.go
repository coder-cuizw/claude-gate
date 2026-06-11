// Package admin 实现管理后台 API（任务书 §6 / §5.7）。
//
// 鉴权：JWT，登录签发，所有 /api/admin/* 校验。资源 CRUD + 统计 + 明细 + 复现。
// 全部依赖通过接口注入，便于用内存实现离线自测。
package admin

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/claude-gate/claude-gate/internal/auth"
	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/gateway"
	"github.com/claude-gate/claude-gate/internal/observ"
	"github.com/claude-gate/claude-gate/internal/store"
)

// Replayer 抽象请求复现能力（由 gateway.Proxy 提供）。
type Replayer interface {
	Replay(ctx context.Context, groupID int64, body []byte, overrideModel string, dryRun bool) (*gateway.ReplayResult, error)
}

// Invalidator 在配置变更时失效热路径缓存（任务书 §5.2）。
type Invalidator interface {
	Invalidate(ctx context.Context, prefix string)
}

// KeyReloader 在上游 Key 变更后重新加载该通道的 Key 选择池，
// 使运行时通过管理 API 新增/启停的 Key 立即生效。
type KeyReloader func(ctx context.Context, channelID int64)

// Deps 是管理 API 的依赖集合。
type Deps struct {
	Store       store.ConfigStore
	Metrics     observ.MetricsReader
	Bodies      observ.BodyStore
	Replayer    Replayer
	Invalidator Invalidator
	KeyReloader KeyReloader
	JWTSecret   string
	JWTTTL      time.Duration
	EncKey      string
	Logger      *slog.Logger
}

// Server 是管理 API 服务。
type Server struct {
	d Deps
}

// NewServer 构造管理 API 服务。
func NewServer(d Deps) *Server {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if d.JWTTTL <= 0 {
		d.JWTTTL = 24 * time.Hour
	}
	return &Server{d: d}
}

// Mount 把 /api/admin/* 路由注册到给定 mux（前缀已含）。
func (s *Server) Mount(mux *http.ServeMux) {
	// 登录无需鉴权
	mux.HandleFunc("POST /api/admin/login", s.handleLogin)

	// 受保护资源
	p := func(pattern string, h http.HandlerFunc) { mux.HandleFunc(pattern, s.auth(h)) }
	p("GET /api/admin/me", s.handleMe)
	p("POST /api/admin/me/password", s.handleChangePassword)

	p("GET /api/admin/channels", s.listChannels)
	p("POST /api/admin/channels", s.createChannel)
	p("PUT /api/admin/channels/{id}", s.updateChannel)
	p("DELETE /api/admin/channels/{id}", s.deleteChannel)
	p("POST /api/admin/channels/{id}/toggle", s.toggleChannel)

	p("GET /api/admin/upstream-keys", s.listUpstreamKeys)
	p("POST /api/admin/upstream-keys", s.createUpstreamKey)
	p("PUT /api/admin/upstream-keys/{id}", s.updateUpstreamKey)
	p("DELETE /api/admin/upstream-keys/{id}", s.deleteUpstreamKey)

	p("GET /api/admin/groups", s.listGroups)
	p("GET /api/admin/groups/{id}", s.getGroup)
	p("POST /api/admin/groups", s.createGroup)
	p("PUT /api/admin/groups/{id}", s.updateGroup)
	p("DELETE /api/admin/groups/{id}", s.deleteGroup)

	p("GET /api/admin/api-keys", s.listAPIKeys)
	p("POST /api/admin/api-keys", s.createAPIKey)
	p("PUT /api/admin/api-keys/{id}", s.updateAPIKey)
	p("DELETE /api/admin/api-keys/{id}", s.deleteAPIKey)
	p("GET /api/admin/api-keys/{id}/reveal", s.revealAPIKey)

	p("GET /api/admin/model-mappings", s.listModelMappings)
	p("POST /api/admin/model-mappings", s.createModelMapping)
	p("DELETE /api/admin/model-mappings/{id}", s.deleteModelMapping)

	p("GET /api/admin/users", s.listUsers)

	p("GET /api/admin/stats/overview", s.statsOverview)
	p("GET /api/admin/stats/timeseries", s.statsTimeseries)
	p("GET /api/admin/stats/errors", s.statsErrors)
	p("GET /api/admin/stats/by-channel", s.statsByChannel)

	p("GET /api/admin/traces", s.listTraces)
	p("GET /api/admin/traces/{id}", s.getTrace)
	p("POST /api/admin/traces/{id}/replay", s.replayTrace)
}

// ---- 鉴权 ----

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct{ Email, Password string }
	if !decode(w, r, &body) {
		return
	}
	u, err := s.d.Store.GetUserByEmail(r.Context(), body.Email)
	if err != nil || !auth.VerifySecret(body.Password, u.PasswordHash) {
		fail(w, http.StatusUnauthorized, "邮箱或密码错误")
		return
	}
	token, err := SignJWT(Claims{Subject: u.Email, Role: u.Role, Expires: time.Now().Add(s.d.JWTTTL).Unix()}, s.d.JWTSecret)
	if err != nil {
		fail(w, http.StatusInternalServerError, "签发 token 失败")
		return
	}
	ok(w, map[string]any{"token": token, "user": map[string]any{"email": u.Email, "role": u.Role}})
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		claims, err := VerifyJWT(token, s.d.JWTSecret)
		if err != nil {
			fail(w, http.StatusUnauthorized, "未授权")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), claimsKey{}, claims)))
	}
}

type claimsKey struct{}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	c, _ := r.Context().Value(claimsKey{}).(*Claims)
	ok(w, map[string]any{"email": c.Subject, "role": c.Role})
}

// handleChangePassword 修改当前登录用户的密码：校验原密码 + 新密码强度后落库。
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if !decode(w, r, &body) {
		return
	}
	if len(body.NewPassword) < 8 {
		fail(w, http.StatusBadRequest, "新密码至少 8 位")
		return
	}
	if body.NewPassword == body.OldPassword {
		fail(w, http.StatusBadRequest, "新密码不能与原密码相同")
		return
	}
	c, _ := r.Context().Value(claimsKey{}).(*Claims)
	u, err := s.d.Store.GetUserByEmail(r.Context(), c.Subject)
	if err != nil {
		fail(w, http.StatusUnauthorized, "用户不存在")
		return
	}
	if !auth.VerifySecret(body.OldPassword, u.PasswordHash) {
		fail(w, http.StatusBadRequest, "原密码不正确")
		return
	}
	if err := s.d.Store.UpdateUserPassword(r.Context(), u.ID, auth.HashSecret(body.NewPassword)); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, map[string]any{"updated": true})
}

// ---- 通道 ----

func (s *Server) listChannels(w http.ResponseWriter, r *http.Request) {
	chs, err := s.d.Store.ListChannels(r.Context())
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, chs)
}

func (s *Server) createChannel(w http.ResponseWriter, r *http.Request) {
	var c domain.UpstreamChannel
	if !decode(w, r, &c) {
		return
	}
	if c.Config == nil {
		c.Config = map[string]any{}
	}
	if err := s.d.Store.CreateChannel(r.Context(), &c); err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, c)
}

func (s *Server) updateChannel(w http.ResponseWriter, r *http.Request) {
	var c domain.UpstreamChannel
	if !decode(w, r, &c) {
		return
	}
	c.ID = pathID(r)
	if err := s.d.Store.UpdateChannel(r.Context(), &c); err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, c)
}

func (s *Server) deleteChannel(w http.ResponseWriter, r *http.Request) {
	if err := s.d.Store.DeleteChannel(r.Context(), pathID(r)); err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, map[string]any{"deleted": true})
}

func (s *Server) toggleChannel(w http.ResponseWriter, r *http.Request) {
	ch, err := s.d.Store.GetChannel(r.Context(), pathID(r))
	if err != nil {
		fail(w, 404, "通道不存在")
		return
	}
	ch.Enabled = !ch.Enabled
	_ = s.d.Store.UpdateChannel(r.Context(), ch)
	ok(w, ch)
}

// ---- 上游 Key ----

func (s *Server) listUpstreamKeys(w http.ResponseWriter, r *http.Request) {
	cid, _ := strconv.ParseInt(r.URL.Query().Get("channel_id"), 10, 64)
	keys, err := s.d.Store.ListUpstreamKeys(r.Context(), cid)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, keys)
}

func (s *Server) createUpstreamKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ChannelID  int64  `json:"channel_id"`
		Name       string `json:"name"`
		Credential string `json:"credential"`
		Status     string `json:"status"`
	}
	if !decode(w, r, &body) {
		return
	}
	enc, _ := auth.Encrypt(body.Credential, s.d.EncKey)
	k := &domain.UpstreamKey{ChannelID: body.ChannelID, Name: body.Name, CredentialEncrypted: enc, Status: domain.KeyStatus(orDefault(body.Status, string(domain.KeyActive)))}
	if err := s.d.Store.CreateUpstreamKey(r.Context(), k); err != nil {
		fail(w, 500, err.Error())
		return
	}
	s.reloadKeys(r.Context(), k.ChannelID)
	ok(w, k)
}

func (s *Server) updateUpstreamKey(w http.ResponseWriter, r *http.Request) {
	k, err := upstreamKeyByID(r.Context(), s.d.Store, pathID(r))
	if err != nil {
		fail(w, 404, "Key 不存在")
		return
	}
	var body struct {
		Name       string `json:"name"`
		Credential string `json:"credential"`
		Status     string `json:"status"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Name != "" {
		k.Name = body.Name
	}
	if body.Credential != "" {
		k.CredentialEncrypted, _ = auth.Encrypt(body.Credential, s.d.EncKey)
	}
	if body.Status != "" {
		k.Status = domain.KeyStatus(body.Status)
	}
	if err := s.d.Store.UpdateUpstreamKey(r.Context(), k); err != nil {
		fail(w, 500, err.Error())
		return
	}
	s.reloadKeys(r.Context(), k.ChannelID)
	ok(w, k)
}

func (s *Server) deleteUpstreamKey(w http.ResponseWriter, r *http.Request) {
	var channelID int64
	if k, err := upstreamKeyByID(r.Context(), s.d.Store, pathID(r)); err == nil {
		channelID = k.ChannelID
	}
	if err := s.d.Store.DeleteUpstreamKey(r.Context(), pathID(r)); err != nil {
		fail(w, 500, err.Error())
		return
	}
	if channelID != 0 {
		s.reloadKeys(r.Context(), channelID)
	}
	ok(w, map[string]any{"deleted": true})
}

func (s *Server) reloadKeys(ctx context.Context, channelID int64) {
	if s.d.KeyReloader != nil {
		s.d.KeyReloader(ctx, channelID)
	}
}

// ---- 分组 ----

func (s *Server) listGroups(w http.ResponseWriter, r *http.Request) {
	gs, err := s.d.Store.ListGroups(r.Context())
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, gs)
}

func (s *Server) getGroup(w http.ResponseWriter, r *http.Request) {
	g, err := s.d.Store.GetGroup(r.Context(), pathID(r))
	if err != nil {
		fail(w, 404, "分组不存在")
		return
	}
	ok(w, g)
}

func (s *Server) createGroup(w http.ResponseWriter, r *http.Request) {
	var g domain.Group
	if !decode(w, r, &g) {
		return
	}
	if err := s.d.Store.CreateGroup(r.Context(), &g); err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, g)
}

func (s *Server) updateGroup(w http.ResponseWriter, r *http.Request) {
	var g domain.Group
	if !decode(w, r, &g) {
		return
	}
	g.ID = pathID(r)
	if err := s.d.Store.UpdateGroup(r.Context(), &g); err != nil {
		fail(w, 500, err.Error())
		return
	}
	s.invalidateGroup(r.Context(), g.ID)
	ok(w, g)
}

func (s *Server) deleteGroup(w http.ResponseWriter, r *http.Request) {
	if err := s.d.Store.DeleteGroup(r.Context(), pathID(r)); err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, map[string]any{"deleted": true})
}

// ---- 客户 API Key ----

func (s *Server) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	ks, err := s.d.Store.ListAPIKeys(r.Context())
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, ks)
}

func (s *Server) createAPIKey(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name      string     `json:"name"`
		GroupID   int64      `json:"group_id"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if !decode(w, r, &body) {
		return
	}
	gen, err := auth.GenerateKey()
	if err != nil {
		fail(w, 500, "生成 Key 失败")
		return
	}
	enc, _ := auth.Encrypt(gen.Plaintext, s.d.EncKey)
	k := &domain.APIKey{KeyPrefix: gen.Prefix, KeyHash: gen.Hash, KeyEncrypted: enc, Name: body.Name, GroupID: body.GroupID, Enabled: true, ExpiresAt: body.ExpiresAt}
	if err := s.d.Store.CreateAPIKey(r.Context(), k); err != nil {
		fail(w, 500, err.Error())
		return
	}
	// 明文随创建返回一次；后续可通过 reveal 重复查看（任务书追加需求）
	ok(w, map[string]any{"api_key": k, "plaintext": gen.Plaintext})
}

func (s *Server) updateAPIKey(w http.ResponseWriter, r *http.Request) {
	k, err := s.d.Store.GetAPIKey(r.Context(), pathID(r))
	if err != nil {
		fail(w, 404, "Key 不存在")
		return
	}
	var body struct {
		Name    string `json:"name"`
		GroupID int64  `json:"group_id"`
		Enabled *bool  `json:"enabled"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Name != "" {
		k.Name = body.Name
	}
	if body.GroupID != 0 {
		k.GroupID = body.GroupID
	}
	if body.Enabled != nil {
		k.Enabled = *body.Enabled
	}
	if err := s.d.Store.UpdateAPIKey(r.Context(), k); err != nil {
		fail(w, 500, err.Error())
		return
	}
	s.invalidatePrefix(r.Context(), k.KeyPrefix)
	ok(w, k)
}

func (s *Server) deleteAPIKey(w http.ResponseWriter, r *http.Request) {
	k, err := s.d.Store.GetAPIKey(r.Context(), pathID(r))
	if err == nil {
		s.invalidatePrefix(r.Context(), k.KeyPrefix)
	}
	if err := s.d.Store.DeleteAPIKey(r.Context(), pathID(r)); err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, map[string]any{"deleted": true})
}

// revealAPIKey 解密返回客户 Key 明文，支持后台重复查看（任务书追加需求）。
func (s *Server) revealAPIKey(w http.ResponseWriter, r *http.Request) {
	k, err := s.d.Store.GetAPIKey(r.Context(), pathID(r))
	if err != nil {
		fail(w, 404, "Key 不存在")
		return
	}
	plain, err := auth.Decrypt(k.KeyEncrypted, s.d.EncKey)
	if err != nil {
		fail(w, 500, "解密失败（可能未配置加密口令）")
		return
	}
	ok(w, map[string]any{"id": k.ID, "name": k.Name, "plaintext": plain})
}

// ---- 模型映射 ----

func (s *Server) listModelMappings(w http.ResponseWriter, r *http.Request) {
	cid, _ := strconv.ParseInt(r.URL.Query().Get("channel_id"), 10, 64)
	ms, err := s.d.Store.ListModelMappings(r.Context(), cid)
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, ms)
}

func (s *Server) createModelMapping(w http.ResponseWriter, r *http.Request) {
	var m domain.ModelMapping
	if !decode(w, r, &m) {
		return
	}
	if err := s.d.Store.CreateModelMapping(r.Context(), &m); err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, m)
}

func (s *Server) deleteModelMapping(w http.ResponseWriter, r *http.Request) {
	if err := s.d.Store.DeleteModelMapping(r.Context(), pathID(r)); err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, map[string]any{"deleted": true})
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	us, err := s.d.Store.ListUsers(r.Context())
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, us)
}

// ---- 统计 ----

func (s *Server) statsOverview(w http.ResponseWriter, r *http.Request) {
	ov, err := s.d.Metrics.Overview(r.Context(), parseStatsQuery(r))
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, ov)
}

func (s *Server) statsTimeseries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	pts, err := s.d.Metrics.Timeseries(r.Context(), parseStatsQuery(r), orDefault(q.Get("metric"), "ttft_p95"), orDefault(q.Get("granularity"), "1m"))
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, pts)
}

func (s *Server) statsErrors(w http.ResponseWriter, r *http.Request) {
	bs, err := s.d.Metrics.Errors(r.Context(), parseStatsQuery(r))
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, bs)
}

func (s *Server) statsByChannel(w http.ResponseWriter, r *http.Request) {
	cs, err := s.d.Metrics.ByChannel(r.Context(), parseStatsQuery(r))
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, cs)
}

// ---- 明细与复现 ----

func (s *Server) listTraces(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	size, _ := strconv.Atoi(q.Get("page_size"))
	gid, _ := strconv.ParseInt(q.Get("group_id"), 10, 64)
	list, err := s.d.Metrics.ListTraces(r.Context(), observ.TraceQuery{
		Status: orDefault(q.Get("status"), "all"), ChannelType: q.Get("channel_type"),
		GroupID: gid, Page: page, PageSize: size,
	})
	if err != nil {
		fail(w, 500, err.Error())
		return
	}
	ok(w, list)
}

func (s *Server) getTrace(w http.ResponseWriter, r *http.Request) {
	rec, err := s.d.Metrics.GetTrace(r.Context(), r.PathValue("id"))
	if err != nil {
		fail(w, 404, "trace 不存在")
		return
	}
	resp := map[string]any{"record": rec}
	// 详情接口拉取 body，设 2s 超时，超时返回 partial（任务书 §5.7）
	if s.d.Bodies != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if rec.RequestBodyS3Key != "" {
			if b, err := s.d.Bodies.Get(ctx, rec.RequestBodyS3Key); err == nil {
				resp["request_body"] = rawOrString(b)
			}
		}
		if rec.ResponseBodyS3Key != "" {
			if b, err := s.d.Bodies.Get(ctx, rec.ResponseBodyS3Key); err == nil {
				resp["response_body"] = rawOrString(b)
			}
		}
	}
	ok(w, resp)
}

func (s *Server) replayTrace(w http.ResponseWriter, r *http.Request) {
	if s.d.Replayer == nil {
		fail(w, 501, "复现能力未启用")
		return
	}
	rec, err := s.d.Metrics.GetTrace(r.Context(), r.PathValue("id"))
	if err != nil {
		fail(w, 404, "trace 不存在")
		return
	}
	var body struct {
		TargetGroupID int64  `json:"target_group_id"`
		DryRun        bool   `json:"dry_run"`
		OverrideModel string `json:"override_model"`
	}
	_ = decodeOptional(r, &body)
	groupID := body.TargetGroupID
	if groupID == 0 {
		groupID = rec.GroupID
	}
	reqBody := []byte("{}")
	if s.d.Bodies != nil && rec.RequestBodyS3Key != "" {
		if b, err := s.d.Bodies.Get(r.Context(), rec.RequestBodyS3Key); err == nil {
			reqBody = b
		}
	}
	res, err := s.d.Replayer.Replay(r.Context(), groupID, reqBody, body.OverrideModel, body.DryRun)
	if err != nil {
		de, _ := domain.AsError(err)
		if de != nil {
			fail(w, de.HTTPStatus, de.UserMessage)
		} else {
			fail(w, 500, err.Error())
		}
		return
	}
	ok(w, res)
}

// ---- 缓存失效 ----

func (s *Server) invalidatePrefix(ctx context.Context, prefix string) {
	if s.d.Invalidator != nil {
		s.d.Invalidator.Invalidate(ctx, prefix)
	}
}

func (s *Server) invalidateGroup(ctx context.Context, groupID int64) {
	// 分组配置变更：失效该分组下所有客户 Key 的前缀缓存。
	if s.d.Invalidator == nil {
		return
	}
	ks, err := s.d.Store.ListAPIKeys(ctx)
	if err != nil {
		return
	}
	for _, k := range ks {
		if k.GroupID == groupID {
			s.d.Invalidator.Invalidate(ctx, k.KeyPrefix)
		}
	}
}

// ---- 辅助 ----

func upstreamKeyByID(ctx context.Context, st store.ConfigStore, id int64) (*domain.UpstreamKey, error) {
	ks, err := st.ListUpstreamKeys(ctx, 0)
	if err != nil {
		return nil, err
	}
	for i := range ks {
		if ks[i].ID == id {
			return &ks[i], nil
		}
	}
	return nil, store.ErrNotFound
}

func parseStatsQuery(r *http.Request) observ.StatsQuery {
	q := r.URL.Query()
	gid, _ := strconv.ParseInt(q.Get("group_id"), 10, 64)
	cid, _ := strconv.ParseInt(q.Get("channel_id"), 10, 64)
	return observ.StatsQuery{From: parseTime(q.Get("from")), To: parseTime(q.Get("to")), GroupID: gid, ChannelID: cid}
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if unix, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(unix, 0)
	}
	return time.Time{}
}

func pathID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		fail(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
		return false
	}
	return true
}

func decodeOptional(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func ok(w http.ResponseWriter, v any) {
	// nil slice → 空数组 []，避免被 encoding/json 序列化成 null，
	// 让前端 .filter/.map 链路免于 TypeError。
	if v != nil {
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Slice && rv.IsNil() {
			v = reflect.MakeSlice(rv.Type(), 0, 0).Interface()
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// rawOrString 把从 S3 取回的 body 安全嵌入响应：合法 JSON 原样内联，
// 否则当字符串（错误请求的原始 body 可能并非合法 JSON）。
func rawOrString(b []byte) any {
	if json.Valid(b) {
		return json.RawMessage(b)
	}
	return string(b)
}
