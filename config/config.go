package config

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

// DatabaseConfig holds MySQL connection configuration
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnectTimeout  time.Duration
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	TLSConfig       string
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Host           string
	Port           int
	Password       string
	SessionTTL     time.Duration
	EmptyTTL       time.Duration
	MaxEntries     int
	PoolSize       int
	MinIdleConns   int
	ConnectTimeout time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
}

// Config holds all configuration for the application
type Config struct {
	Database DatabaseConfig
	Redis    RedisConfig
	Server   ServerConfig
	Logging  LoggingConfig
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port string
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level      string
	Format     string
	EnableJSON bool
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnvInt("DB_PORT", 3306),
			User:            getEnv("DB_USER", "root"),
			Password:        getEnv("DB_PASSWORD", ""),
			Database:        getEnv("DB_NAME", "agent_sessions"),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 20),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: parseDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
			ConnectTimeout:  parseDuration("DB_CONNECT_TIMEOUT", 5*time.Second),
			ReadTimeout:     parseDuration("DB_READ_TIMEOUT", 3*time.Second),
			WriteTimeout:    parseDuration("DB_WRITE_TIMEOUT", 3*time.Second),
			TLSConfig:       "",
		},
		Redis: RedisConfig{
			Host:           getEnv("REDIS_HOST", "localhost"),
			Port:           getEnvInt("REDIS_PORT", 6379),
			Password:       getEnv("REDIS_PASSWORD", ""),
			SessionTTL:     parseDuration("REDIS_SESSION_TTL", 24*time.Hour),
			EmptyTTL:       parseDuration("REDIS_EMPTY_TTL", 60*time.Second),
			MaxEntries:     getEnvInt("REDIS_MAX_ENTRIES", 10000),
			PoolSize:       getEnvInt("REDIS_POOL_SIZE", 10),
			MinIdleConns:   getEnvInt("REDIS_MIN_IDLE_CONNS", 2),
			ConnectTimeout: parseDuration("REDIS_CONNECT_TIMEOUT", 5*time.Second),
			ReadTimeout:    parseDuration("REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout:   parseDuration("REDIS_WRITE_TIMEOUT", 3*time.Second),
		},
		Server: ServerConfig{
			Port: getEnv("PORT", "8888"),
		},
		Logging: LoggingConfig{
			Level:      getEnv("LOG_LEVEL", "info"),
			Format:     getEnv("LOG_FORMAT", "text"),
			EnableJSON: getEnv("LOG_ENABLE_JSON", "false") == "true",
		},
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate required database parameters
	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if c.Database.Database == "" {
		return fmt.Errorf("database name is required")
	}

	// Validate port numbers
	if c.Database.Port < 1 || c.Database.Port > 65535 {
		return fmt.Errorf("database port must be between 1 and 65535")
	}
	if c.Redis.Port < 1 || c.Redis.Port > 65535 {
		return fmt.Errorf("redis port must be between 1 and 65535")
	}

	// Validate timeout values
	if c.Database.ConnectTimeout <= 0 {
		return fmt.Errorf("database connect timeout must be positive")
	}
	if c.Database.ReadTimeout <= 0 {
		return fmt.Errorf("database read timeout must be positive")
	}
	if c.Database.WriteTimeout <= 0 {
		return fmt.Errorf("database write timeout must be positive")
	}

	// Validate Redis timeout values
	if c.Redis.ConnectTimeout <= 0 {
		return fmt.Errorf("redis connect timeout must be positive")
	}
	if c.Redis.ReadTimeout <= 0 {
		return fmt.Errorf("redis read timeout must be positive")
	}
	if c.Redis.WriteTimeout <= 0 {
		return fmt.Errorf("redis write timeout must be positive")
	}

	// Validate session TTL
	if c.Redis.SessionTTL <= 0 {
		return fmt.Errorf("redis session TTL must be positive")
	}

	// Validate empty TTL
	if c.Redis.EmptyTTL <= 0 {
		return fmt.Errorf("redis empty TTL must be positive")
	}

	// Validate pool sizes
	if c.Database.MaxOpenConns < 1 {
		return fmt.Errorf("database max open connections must be at least 1")
	}
	if c.Database.MaxIdleConns < 0 {
		return fmt.Errorf("database max idle connections must be non-negative")
	}
	if c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		return fmt.Errorf("database max idle connections cannot exceed max open connections")
	}

	// Validate Redis pool sizes
	if c.Redis.PoolSize < 1 {
		return fmt.Errorf("redis pool size must be at least 1")
	}
	if c.Redis.MinIdleConns < 0 {
		return fmt.Errorf("redis min idle connections must be non-negative")
	}

	// Validate logging configuration
	if c.Logging.Level != "debug" && c.Logging.Level != "info" && c.Logging.Level != "warn" && c.Logging.Level != "error" {
		return fmt.Errorf("log level must be debug, info, warn, or error")
	}
	if c.Logging.Format != "text" && c.Logging.Format != "json" {
		return fmt.Errorf("log format must be text or json")
	}

	return nil
}

// LogConfiguration logs the current configuration
func (c *Config) LogConfiguration() {
	log.Println("=== Configuration ===")
	log.Printf("Server Port: %s", c.Server.Port)
	log.Printf("Log Level: %s", c.Logging.Level)
	log.Printf("Log Format: %s", c.Logging.Format)

	log.Println("--- Database ---")
	log.Printf("Host: %s:%d", c.Database.Host, c.Database.Port)
	log.Printf("Database: %s", c.Database.Database)
	log.Printf("User: %s", c.Database.User)
	log.Printf("Max Open Conns: %d", c.Database.MaxOpenConns)
	log.Printf("Max Idle Conns: %d", c.Database.MaxIdleConns)
	log.Printf("Conn Max Lifetime: %v", c.Database.ConnMaxLifetime)
	log.Printf("Connect Timeout: %v", c.Database.ConnectTimeout)
	log.Printf("Read Timeout: %v", c.Database.ReadTimeout)
	log.Printf("Write Timeout: %v", c.Database.WriteTimeout)

	log.Println("--- Redis ---")
	log.Printf("Host: %s:%d", c.Redis.Host, c.Redis.Port)
	log.Printf("Session TTL: %v", c.Redis.SessionTTL)
	log.Printf("Empty TTL: %v", c.Redis.EmptyTTL)
	log.Printf("Max Entries: %d", c.Redis.MaxEntries)
	log.Printf("Pool Size: %d", c.Redis.PoolSize)
	log.Printf("Min Idle Conns: %d", c.Redis.MinIdleConns)
	log.Printf("Connect Timeout: %v", c.Redis.ConnectTimeout)
	log.Printf("Read Timeout: %v", c.Redis.ReadTimeout)
	log.Printf("Write Timeout: %v", c.Redis.WriteTimeout)
	log.Println("===================")
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an environment variable as int or returns a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		// Try to parse as int
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// parseDuration gets an environment variable as duration or returns a default value
func parseDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		// Try to parse as duration
		if parsed, err := time.ParseDuration(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// InitDB initializes a MySQL database connection
func (c *DatabaseConfig) InitDB() (*sql.DB, error) {
	// Build DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4",
		c.User, c.Password, c.Host, c.Port, c.Database)

	// Add timeout parameters
	dsn += fmt.Sprintf("&timeout=%s&readTimeout=%s&writeTimeout=%s",
		c.ConnectTimeout, c.ReadTimeout, c.WriteTimeout)

	// Initialize MySQL connection
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(c.MaxOpenConns)
	db.SetMaxIdleConns(c.MaxIdleConns)
	db.SetConnMaxLifetime(c.ConnMaxLifetime)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Printf("Successfully connected to MySQL database at %s:%d/%s", c.Host, c.Port, c.Database)

	return db, nil
}

// InitRedis initializes a Redis client
func (c *RedisConfig) InitRedis() (*redis.Client, error) {
	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", c.Host, c.Port),
		Password:     c.Password,
		PoolSize:     c.PoolSize,
		MinIdleConns: c.MinIdleConns,
	})

	// Test connection
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Printf("Successfully connected to Redis at %s:%d", c.Host, c.Port)

	return client, nil
}
