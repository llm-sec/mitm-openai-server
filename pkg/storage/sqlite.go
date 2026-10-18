package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// AnonymousUsername 匿名用户用户名
// 未认证的API请求（如独立模式下直接访问/v1接口）会归一到该用户，满足外键约束
const AnonymousUsername = "anonymous"

// SQLiteStorage 实现Storage接口，使用SQLite作为存储后端
type SQLiteStorage struct {
	db         *sql.DB
	dbPath     string
	dataFolder string
}

// NewSQLiteStorage 创建一个新的SQLite存储实例
func NewSQLiteStorage(dataFolder string) (*SQLiteStorage, error) {
	// 如果未提供路径，使用默认位置
	if dataFolder == "" {
		// 获取当前目录作为默认存储位置
		currentDir, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("获取当前工作目录失败: %w", err)
		}
		dataFolder = filepath.Join(currentDir, "data")
	}

	// 确保数据文件夹存在
	if err := os.MkdirAll(dataFolder, 0755); err != nil {
		return nil, fmt.Errorf("创建数据文件夹失败: %w", err)
	}

	dbPath := filepath.Join(dataFolder, "mitm_server.db")

	// 确保数据库文件所在目录存在
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开SQLite数据库失败: %w", err)
	}

	// 验证连接
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("无法连接到SQLite数据库: %w", err)
	}

	storage := &SQLiteStorage{
		db:         db,
		dbPath:     dbPath,
		dataFolder: dataFolder,
	}

	// 初始化数据库表
	if err := storage.InitDatabase(); err != nil {
		db.Close()
		return nil, fmt.Errorf("初始化数据库失败: %w", err)
	}

	return storage, nil
}

// InitDatabase 初始化数据库表结构
func (s *SQLiteStorage) InitDatabase() error {
	// 开启外键约束
	_, err := s.db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		return fmt.Errorf("启用外键约束失败: %w", err)
	}

	// 创建用户表
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL,
			user_type TEXT NOT NULL DEFAULT 'user',
			is_active BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_login_at DATETIME,
			
			-- 代理配置
			proxy_enabled BOOLEAN NOT NULL DEFAULT 0,
			proxy_target_url TEXT,
			proxy_auth_type TEXT,
			proxy_username TEXT,
			proxy_password TEXT,
			proxy_token TEXT,
			
			-- 其他配置
			storage_path TEXT,
			max_requests INTEGER NOT NULL DEFAULT 10000,
			data_retention_days INTEGER NOT NULL DEFAULT 30
		)
	`)
	if err != nil {
		return fmt.Errorf("创建用户表失败: %w", err)
	}

	// 创建请求表
	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS requests (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			method TEXT NOT NULL,
			path TEXT NOT NULL,
			timestamp DATETIME NOT NULL,
			headers TEXT NOT NULL,
			query TEXT NOT NULL,
			body TEXT,
			client_ip TEXT NOT NULL,
			ip_address TEXT NOT NULL,
			response TEXT,
			metadata TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("创建请求表失败: %w", err)
	}

	// 创建索引提升查询性能
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_requests_user_id ON requests(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_requests_timestamp ON requests(timestamp)",
		"CREATE INDEX IF NOT EXISTS idx_requests_user_timestamp ON requests(user_id, timestamp)",
		"CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)",
	}

	for _, indexSQL := range indexes {
		_, err = s.db.Exec(indexSQL)
		if err != nil {
			return fmt.Errorf("创建索引失败: %w", err)
		}
	}

	return nil
}

// ==================== 用户管理方法 ====================

// CreateUser 创建新用户
func (s *SQLiteStorage) CreateUser(user *User) error {
	query := `
		INSERT INTO users (
			username, password, user_type, is_active,
			proxy_enabled, proxy_target_url, proxy_auth_type, 
			proxy_username, proxy_password, proxy_token,
			storage_path, max_requests, data_retention_days
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := s.db.Exec(query,
		user.Username, user.Password, user.UserType, user.IsActive,
		user.ProxyEnabled, user.ProxyTargetURL, user.ProxyAuthType,
		user.ProxyUsername, user.ProxyPassword, user.ProxyToken,
		user.StoragePath, user.MaxRequests, user.DataRetentionDays,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("用户名已存在")
		}
		return fmt.Errorf("创建用户失败: %w", err)
	}

	// 获取插入的用户ID
	userID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("获取用户ID失败: %w", err)
	}
	user.ID = userID

	return nil
}

// GetUserByUsername 根据用户名获取用户信息
func (s *SQLiteStorage) GetUserByUsername(username string) (*User, error) {
	query := `
		SELECT id, username, password, user_type, is_active,
			   created_at, updated_at, last_login_at,
			   proxy_enabled, proxy_target_url, proxy_auth_type,
			   proxy_username, proxy_password, proxy_token,
			   storage_path, max_requests, data_retention_days
		FROM users WHERE username = ?
	`

	var user User
	var createdAt, updatedAt, lastLoginAt sql.NullString

	err := s.db.QueryRow(query, username).Scan(
		&user.ID, &user.Username, &user.Password,
		&user.UserType, &user.IsActive, &createdAt, &updatedAt, &lastLoginAt,
		&user.ProxyEnabled, &user.ProxyTargetURL, &user.ProxyAuthType,
		&user.ProxyUsername, &user.ProxyPassword, &user.ProxyToken,
		&user.StoragePath, &user.MaxRequests, &user.DataRetentionDays,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("用户不存在")
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 解析时间字段
	if createdAt.Valid {
		if t, err := time.Parse("2006-01-02 15:04:05", createdAt.String); err == nil {
			user.CreatedAt = t
		}
	}
	if updatedAt.Valid {
		if t, err := time.Parse("2006-01-02 15:04:05", updatedAt.String); err == nil {
			user.UpdatedAt = t
		}
	}
	if lastLoginAt.Valid {
		if t, err := time.Parse("2006-01-02 15:04:05", lastLoginAt.String); err == nil {
			user.LastLoginAt = t
		}
	}

	return &user, nil
}

// GetUserByID 根据用户ID获取用户信息
func (s *SQLiteStorage) GetUserByID(userID int64) (*User, error) {
	query := `
		SELECT id, username, password, user_type, is_active,
			   created_at, updated_at, last_login_at,
			   proxy_enabled, proxy_target_url, proxy_auth_type,
			   proxy_username, proxy_password, proxy_token,
			   storage_path, max_requests, data_retention_days
		FROM users WHERE id = ?
	`

	var user User
	var createdAt, updatedAt, lastLoginAt sql.NullString

	err := s.db.QueryRow(query, userID).Scan(
		&user.ID, &user.Username, &user.Password,
		&user.UserType, &user.IsActive, &createdAt, &updatedAt, &lastLoginAt,
		&user.ProxyEnabled, &user.ProxyTargetURL, &user.ProxyAuthType,
		&user.ProxyUsername, &user.ProxyPassword, &user.ProxyToken,
		&user.StoragePath, &user.MaxRequests, &user.DataRetentionDays,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("用户不存在")
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 解析时间字段（同上）
	if createdAt.Valid {
		if t, err := time.Parse("2006-01-02 15:04:05", createdAt.String); err == nil {
			user.CreatedAt = t
		}
	}
	if updatedAt.Valid {
		if t, err := time.Parse("2006-01-02 15:04:05", updatedAt.String); err == nil {
			user.UpdatedAt = t
		}
	}
	if lastLoginAt.Valid {
		if t, err := time.Parse("2006-01-02 15:04:05", lastLoginAt.String); err == nil {
			user.LastLoginAt = t
		}
	}

	return &user, nil
}

// UpdateUser 更新用户信息
func (s *SQLiteStorage) UpdateUser(user *User) error {
	query := `
		UPDATE users SET 
			username = ?, password = ?, user_type = ?, is_active = ?,
			updated_at = CURRENT_TIMESTAMP,
			proxy_enabled = ?, proxy_target_url = ?, proxy_auth_type = ?,
			proxy_username = ?, proxy_password = ?, proxy_token = ?,
			storage_path = ?, max_requests = ?, data_retention_days = ?
		WHERE id = ?
	`

	_, err := s.db.Exec(query,
		user.Username, user.Password, user.UserType, user.IsActive,
		user.ProxyEnabled, user.ProxyTargetURL, user.ProxyAuthType,
		user.ProxyUsername, user.ProxyPassword, user.ProxyToken,
		user.StoragePath, user.MaxRequests, user.DataRetentionDays,
		user.ID,
	)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("用户名已存在")
		}
		return fmt.Errorf("更新用户失败: %w", err)
	}

	return nil
}

// UpdateUserConfig 更新用户配置
func (s *SQLiteStorage) UpdateUserConfig(userID int64, config *UserConfig) error {
	query := `
		UPDATE users SET 
			proxy_enabled = ?, proxy_target_url = ?, proxy_auth_type = ?,
			proxy_username = ?, proxy_password = ?, proxy_token = ?,
			max_requests = ?, data_retention_days = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := s.db.Exec(query,
		config.ProxyEnabled, config.ProxyTargetURL, config.ProxyAuthType,
		config.ProxyUsername, config.ProxyPassword, config.ProxyToken,
		config.MaxRequests, config.DataRetentionDays,
		userID,
	)

	if err != nil {
		return fmt.Errorf("更新用户配置失败: %w", err)
	}

	return nil
}

// UpdateUserLastLogin 更新用户最后登录时间
func (s *SQLiteStorage) UpdateUserLastLogin(userID int64) error {
	query := "UPDATE users SET last_login_at = CURRENT_TIMESTAMP WHERE id = ?"
	_, err := s.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("更新最后登录时间失败: %w", err)
	}
	return nil
}

// ValidateUserCredentials 验证用户凭据
func (s *SQLiteStorage) ValidateUserCredentials(username, password string) (*User, error) {
	user, err := s.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}

	if !user.IsActive {
		return nil, fmt.Errorf("用户已被禁用")
	}

	if user.Password != password {
		return nil, fmt.Errorf("密码错误")
	}

	return user, nil
}

// ListUsers 获取用户列表（仅管理员可用）
func (s *SQLiteStorage) ListUsers(limit, offset int) ([]*User, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT id, username, user_type, is_active,
			   created_at, updated_at, last_login_at
		FROM users 
		ORDER BY created_at DESC 
		LIMIT ? OFFSET ?
	`

	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("查询用户列表失败: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		var createdAt, updatedAt, lastLoginAt sql.NullString

		err := rows.Scan(
			&user.ID, &user.Username, &user.UserType, &user.IsActive,
			&createdAt, &updatedAt, &lastLoginAt,
		)
		if err != nil {
			log.Printf("扫描用户行失败: %v", err)
			continue
		}

		// 解析时间字段
		if createdAt.Valid {
			if t, err := time.Parse("2006-01-02 15:04:05", createdAt.String); err == nil {
				user.CreatedAt = t
			}
		}
		if updatedAt.Valid {
			if t, err := time.Parse("2006-01-02 15:04:05", updatedAt.String); err == nil {
				user.UpdatedAt = t
			}
		}
		if lastLoginAt.Valid {
			if t, err := time.Parse("2006-01-02 15:04:05", lastLoginAt.String); err == nil {
				user.LastLoginAt = t
			}
		}

		users = append(users, &user)
	}

	return users, nil
}

// ==================== 请求管理方法（用户隔离） ====================

// SaveRequest 保存请求到SQLite数据库（需要用户ID）
func (s *SQLiteStorage) SaveRequest(userID int64, req *Request) error {
	// 未认证/匿名的请求（userID<=0）归一到匿名用户，满足外键约束
	if userID <= 0 {
		anonymousID, err := s.ensureAnonymousUser()
		if err != nil {
			return fmt.Errorf("获取匿名用户失败: %w", err)
		}
		userID = anonymousID
	}

	// 设置请求的用户ID
	req.UserID = userID

	headersJSON, err := json.Marshal(req.Headers)
	if err != nil {
		return fmt.Errorf("序列化请求头失败: %w", err)
	}

	queryJSON, err := json.Marshal(req.Query)
	if err != nil {
		return fmt.Errorf("序列化查询参数失败: %w", err)
	}

	var bodyJSON []byte
	if req.Body != nil {
		bodyJSON, err = json.Marshal(req.Body)
		if err != nil {
			return fmt.Errorf("序列化请求体失败: %w", err)
		}
	}

	var responseJSON []byte
	if req.Response != nil {
		responseJSON, err = json.Marshal(req.Response)
		if err != nil {
			return fmt.Errorf("序列化响应失败: %w", err)
		}
	}

	var metadataJSON []byte
	if req.Metadata != nil {
		metadataJSON, err = json.Marshal(req.Metadata)
		if err != nil {
			return fmt.Errorf("序列化元数据失败: %w", err)
		}
	}

	// 获取客户端IP地址，兼容IPAddress和ClientIP字段
	clientIP := req.ClientIP
	if clientIP == "" && req.IPAddress != "" {
		clientIP = req.IPAddress
	}

	_, err = s.db.Exec(
		`INSERT OR REPLACE INTO requests 
		(id, user_id, method, path, timestamp, headers, query, body, client_ip, ip_address, response, metadata) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ID, userID, req.Method, req.Path, req.Timestamp,
		string(headersJSON), string(queryJSON), string(bodyJSON),
		clientIP, clientIP, string(responseJSON), string(metadataJSON),
	)

	if err != nil {
		return fmt.Errorf("保存请求到数据库失败: %w", err)
	}

	return nil
}

// ensureAnonymousUser 确保存在匿名用户并返回其ID（懒创建，线程安全）
func (s *SQLiteStorage) ensureAnonymousUser() (int64, error) {
	// 先尝试查询已存在的匿名用户
	var id int64
	err := s.db.QueryRow(`SELECT id FROM users WHERE username = ?`, AnonymousUsername).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("查询匿名用户失败: %w", err)
	}

	// 不存在则创建；并发场景下可能同时创建，冲突时回退为查询
	u := &User{
		Username:         AnonymousUsername,
		Password:         "", // 匿名用户无凭据
		UserType:         "user",
		IsActive:         true,
		MaxRequests:      10000,
		DataRetentionDays: 30,
	}
	if err := s.CreateUser(u); err != nil {
		if strings.Contains(err.Error(), "用户名已存在") {
			err2 := s.db.QueryRow(`SELECT id FROM users WHERE username = ?`, AnonymousUsername).Scan(&id)
			if err2 != nil {
				return 0, fmt.Errorf("创建匿名用户失败: %w", err)
			}
			return id, nil
		}
		return 0, fmt.Errorf("创建匿名用户失败: %w", err)
	}
	return u.ID, nil
}

// GetRequestByID 根据ID获取特定用户的请求
func (s *SQLiteStorage) GetRequestByID(userID int64, id string) (*Request, error) {
	var (
		req          Request
		timestampStr string
		headersJSON  string
		queryJSON    string
		bodyJSON     sql.NullString
		responseJSON sql.NullString
		metadataJSON sql.NullString
		ipAddress    string
	)

	err := s.db.QueryRow(
		`SELECT id, user_id, method, path, timestamp, headers, query, body, client_ip, response, metadata
		FROM requests WHERE id = ? AND user_id = ?`, id, userID,
	).Scan(
		&req.ID, &req.UserID, &req.Method, &req.Path, &timestampStr,
		&headersJSON, &queryJSON, &bodyJSON, &ipAddress, &responseJSON, &metadataJSON,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("未找到ID为 %s 的请求或无权限访问", id)
		}
		return nil, fmt.Errorf("查询请求失败: %w", err)
	}

	// 设置IP地址字段
	req.ClientIP = ipAddress
	req.IPAddress = ipAddress

	// 解析时间戳
	timestamp, err := time.Parse("2006-01-02 15:04:05", timestampStr)
	if err != nil {
		req.Timestamp = time.Now()
	} else {
		req.Timestamp = timestamp
	}

	// 解析JSON字段
	if err := json.Unmarshal([]byte(headersJSON), &req.Headers); err != nil {
		return nil, fmt.Errorf("解析请求头JSON失败: %w", err)
	}

	if err := json.Unmarshal([]byte(queryJSON), &req.Query); err != nil {
		return nil, fmt.Errorf("解析查询参数JSON失败: %w", err)
	}

	if bodyJSON.Valid && bodyJSON.String != "" {
		if err := json.Unmarshal([]byte(bodyJSON.String), &req.Body); err != nil {
			return nil, fmt.Errorf("解析请求体JSON失败: %w", err)
		}
	}

	if responseJSON.Valid && responseJSON.String != "" {
		var resp ProxyResponse
		if err := json.Unmarshal([]byte(responseJSON.String), &resp); err != nil {
			return nil, fmt.Errorf("解析响应JSON失败: %w", err)
		}
		req.Response = &resp
	}

	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &req.Metadata); err != nil {
			return nil, fmt.Errorf("解析元数据JSON失败: %w", err)
		}
	}

	return &req, nil
}

// GetUserRequests 获取指定用户的所有请求
func (s *SQLiteStorage) GetUserRequests(userID int64, limit, offset int) ([]*Request, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.Query(
		`SELECT id, user_id, method, path, timestamp, headers, query, body, client_ip, response, metadata
		FROM requests WHERE user_id = ? ORDER BY timestamp DESC LIMIT ? OFFSET ?`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("查询用户请求失败: %w", err)
	}
	defer rows.Close()

	var requests []*Request
	var parseErrors []string

	for rows.Next() {
		var (
			req          Request
			timestampStr string
			headersJSON  string
			queryJSON    string
			bodyJSON     sql.NullString
			responseJSON sql.NullString
			metadataJSON sql.NullString
			ipAddress    string
		)

		err := rows.Scan(
			&req.ID, &req.UserID, &req.Method, &req.Path, &timestampStr,
			&headersJSON, &queryJSON, &bodyJSON, &ipAddress, &responseJSON, &metadataJSON,
		)
		if err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("扫描请求行失败(ID可能未知): %v", err))
			continue
		}

		// 设置IP地址字段
		req.ClientIP = ipAddress
		req.IPAddress = ipAddress

		// 解析时间戳
		timestamp, err := time.Parse("2006-01-02 15:04:05", timestampStr)
		if err != nil {
			req.Timestamp = time.Now()
		} else {
			req.Timestamp = timestamp
		}

		// 解析JSON字段，出错时继续处理
		validRequest := true

		if err := json.Unmarshal([]byte(headersJSON), &req.Headers); err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("解析请求头JSON失败(ID:%s): %v", req.ID, err))
			validRequest = false
		}

		if err := json.Unmarshal([]byte(queryJSON), &req.Query); err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("解析查询参数JSON失败(ID:%s): %v", req.ID, err))
			validRequest = false
		}

		if validRequest {
			if bodyJSON.Valid && bodyJSON.String != "" {
				if err := json.Unmarshal([]byte(bodyJSON.String), &req.Body); err != nil {
					parseErrors = append(parseErrors, fmt.Sprintf("解析请求体JSON失败(ID:%s): %v", req.ID, err))
				}
			}

			if responseJSON.Valid && responseJSON.String != "" {
				var resp ProxyResponse
				if err := json.Unmarshal([]byte(responseJSON.String), &resp); err != nil {
					parseErrors = append(parseErrors, fmt.Sprintf("解析响应JSON失败(ID:%s): %v", req.ID, err))
				} else {
					req.Response = &resp
				}
			}

			if metadataJSON.Valid && metadataJSON.String != "" {
				if err := json.Unmarshal([]byte(metadataJSON.String), &req.Metadata); err != nil {
					parseErrors = append(parseErrors, fmt.Sprintf("解析元数据JSON失败(ID:%s): %v", req.ID, err))
				}
			}

			requests = append(requests, &req)
		}
	}

	// 如果有解析错误，记录但不影响返回
	if len(parseErrors) > 0 {
		log.Printf("GetUserRequests 警告: 有 %d 个解析错误，但成功获取了 %d 个请求", len(parseErrors), len(requests))
		for _, errMsg := range parseErrors {
			log.Printf("解析错误: %s", errMsg)
		}
	}

	return requests, nil
}

// DeleteRequest 删除指定用户的特定请求
func (s *SQLiteStorage) DeleteRequest(userID int64, id string) error {
	result, err := s.db.Exec("DELETE FROM requests WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return fmt.Errorf("删除请求失败: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取删除结果失败: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("未找到ID为 %s 的请求或无权限删除", id)
	}

	return nil
}

// DeleteAllUserRequests 删除指定用户的所有请求
func (s *SQLiteStorage) DeleteAllUserRequests(userID int64) error {
	_, err := s.db.Exec("DELETE FROM requests WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("删除用户所有请求失败: %w", err)
	}
	return nil
}

// GetUserStats 获取指定用户的统计信息
func (s *SQLiteStorage) GetUserStats(userID int64) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总请求数
	var totalRequests int
	err := s.db.QueryRow("SELECT COUNT(*) FROM requests WHERE user_id = ?", userID).Scan(&totalRequests)
	if err != nil {
		return nil, fmt.Errorf("获取总请求数失败: %w", err)
	}
	stats["total_requests"] = totalRequests

	if totalRequests == 0 {
		return stats, nil
	}

	// 按方法统计
	methodRows, err := s.db.Query("SELECT method, COUNT(*) FROM requests WHERE user_id = ? GROUP BY method", userID)
	if err == nil {
		methodCounts := make(map[string]int)
		for methodRows.Next() {
			var method string
			var count int
			if methodRows.Scan(&method, &count) == nil {
				methodCounts[method] = count
			}
		}
		methodRows.Close()
		stats["method_counts"] = methodCounts
	}

	// 最新请求时间
	var latestTimestamp sql.NullString
	err = s.db.QueryRow("SELECT MAX(timestamp) FROM requests WHERE user_id = ?", userID).Scan(&latestTimestamp)
	if err == nil && latestTimestamp.Valid {
		stats["latest_request"] = latestTimestamp.String
	}

	return stats, nil
}

// ExportUserRequests 导出指定用户的请求数据
func (s *SQLiteStorage) ExportUserRequests(userID int64) (string, error) {
	requests, err := s.GetUserRequests(userID, 0, 0) // 获取所有请求
	if err != nil {
		return "", fmt.Errorf("获取用户请求失败: %w", err)
	}

	// 创建导出文件
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("user_%d_requests_%s.json", userID, timestamp)
	filepath := filepath.Join(s.dataFolder, filename)

	file, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("创建导出文件失败: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(requests); err != nil {
		return "", fmt.Errorf("编码导出数据失败: %w", err)
	}

	return filepath, nil
}

// ==================== 管理员专用方法 ====================

// GetAllRequests 获取所有请求（仅管理员可用）
func (s *SQLiteStorage) GetAllRequests(limit, offset int) ([]*Request, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.Query(
		`SELECT id, user_id, method, path, timestamp, headers, query, body, client_ip, response, metadata
		FROM requests ORDER BY timestamp DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("查询所有请求失败: %w", err)
	}
	defer rows.Close()

	var requests []*Request
	for rows.Next() {
		var (
			req          Request
			timestampStr string
			headersJSON  string
			queryJSON    string
			bodyJSON     sql.NullString
			responseJSON sql.NullString
			metadataJSON sql.NullString
			ipAddress    string
		)

		err := rows.Scan(
			&req.ID, &req.UserID, &req.Method, &req.Path, &timestampStr,
			&headersJSON, &queryJSON, &bodyJSON, &ipAddress, &responseJSON, &metadataJSON,
		)
		if err != nil {
			continue
		}

		// 设置IP地址字段
		req.ClientIP = ipAddress
		req.IPAddress = ipAddress

		// 解析时间戳
		if timestamp, err := time.Parse("2006-01-02 15:04:05", timestampStr); err == nil {
			req.Timestamp = timestamp
		} else {
			req.Timestamp = time.Now()
		}

		// 解析JSON字段
		json.Unmarshal([]byte(headersJSON), &req.Headers)
		json.Unmarshal([]byte(queryJSON), &req.Query)

		if bodyJSON.Valid && bodyJSON.String != "" {
			json.Unmarshal([]byte(bodyJSON.String), &req.Body)
		}

		if responseJSON.Valid && responseJSON.String != "" {
			var resp ProxyResponse
			if json.Unmarshal([]byte(responseJSON.String), &resp) == nil {
				req.Response = &resp
			}
		}

		if metadataJSON.Valid && metadataJSON.String != "" {
			json.Unmarshal([]byte(metadataJSON.String), &req.Metadata)
		}

		requests = append(requests, &req)
	}

	return requests, nil
}

// DeleteAllRequests 删除所有请求（仅管理员可用）
func (s *SQLiteStorage) DeleteAllRequests() error {
	_, err := s.db.Exec("DELETE FROM requests")
	if err != nil {
		return fmt.Errorf("删除所有请求失败: %w", err)
	}
	return nil
}

// GetStats 获取全局统计信息（仅管理员可用）
func (s *SQLiteStorage) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 总请求数
	var totalRequests int
	err := s.db.QueryRow("SELECT COUNT(*) FROM requests").Scan(&totalRequests)
	if err != nil {
		return nil, fmt.Errorf("获取总请求数失败: %w", err)
	}
	stats["total_requests"] = totalRequests

	// 用户数量
	var totalUsers int
	err = s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)
	if err == nil {
		stats["total_users"] = totalUsers
	}

	if totalRequests == 0 {
		return stats, nil
	}

	// 按方法统计
	methodRows, err := s.db.Query("SELECT method, COUNT(*) FROM requests GROUP BY method")
	if err == nil {
		methodCounts := make(map[string]int)
		for methodRows.Next() {
			var method string
			var count int
			if methodRows.Scan(&method, &count) == nil {
				methodCounts[method] = count
			}
		}
		methodRows.Close()
		stats["method_counts"] = methodCounts
	}

	return stats, nil
}

// ExportRequests 导出所有请求数据（仅管理员可用）
func (s *SQLiteStorage) ExportRequests() (string, error) {
	requests, err := s.GetAllRequests(0, 0) // 获取所有请求
	if err != nil {
		return "", fmt.Errorf("获取所有请求失败: %w", err)
	}

	// 创建导出文件
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("all_requests_%s.json", timestamp)
	filepath := filepath.Join(s.dataFolder, filename)

	file, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("创建导出文件失败: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(requests); err != nil {
		return "", fmt.Errorf("编码导出数据失败: %w", err)
	}

	return filepath, nil
}

// Close 关闭数据库连接
func (s *SQLiteStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
