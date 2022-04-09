package cache

import (
	"errors"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"time"
)

var redisPool map[int]*redis.Pool

func init() {
	redisPool = make(map[int]*redis.Pool)
}

type RedisConfig struct {
	Port          string
	Password      string
	OpMaxIdle     int
	OpMaxActive   int
	OpIdleTimeout int
}

type Option func(*redis.Pool)

// Maximum number of idle connections in the pool.
func MaxIdle(maxIdle int) Option {
	return func(r *redis.Pool) {
		if r.MaxIdle > 0 {
			r.MaxIdle = maxIdle
		}
	}
}

// Maximum number of connections allocated by the pool at a given time.
// When zero, there is no limit on the number of connections in the pool.
func MaxActive(maxActive int) Option {
	return func(r *redis.Pool) {
		if maxActive > 0 {
			r.MaxActive = maxActive
		}
	}
}

// Close connections after remaining idle for this duration. If the value
// is zero, then idle connections are not closed. Applications should set
// the timeout to a value less than the server's timeout.
func IdleTimeout(idleTimeout int) Option {
	return func(r *redis.Pool) {
		if r.IdleTimeout > 0 {
			r.IdleTimeout = time.Duration(idleTimeout) * time.Second
		}
	}
}

func InitRedis(address, port, password string, db int, configs ...func(rc *redis.Pool)) error {

	// 测试redis 是否可以的连通性
	_, err := redis.Dial("tcp", fmt.Sprintf("%s:%s", address, port), redis.DialDatabase(db), redis.DialPassword(password))
	if err != nil {
		return err
	}

	redisPool[db] = &redis.Pool{
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", fmt.Sprintf("%s:%s", address, port), redis.DialDatabase(db), redis.DialPassword(password))
			if err != nil {
				return nil, err
			}
			if password != "" {
				if _, err := c.Do("AUTH", password); err != nil {
					c.Close()
					return nil, err
				}
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}

	for _, c := range configs {
		c(redisPool[db])
	}
	return nil
}

func GetRedisPoll(db int) (*redis.Pool, error) {
	if redis, ok := redisPool[db]; ok {
		return redis, nil
	} else {
		return nil, errors.New("redis DataBase not exist")
	}
}
