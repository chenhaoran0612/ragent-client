package gredis

import (
	db "amenity.com/ragent-client/kit/cache"
	"github.com/gomodule/redigo/redis"
)

type cache struct {
	db int
}

var RedisCache = 1

//Cache ...
type Cache interface {
	SetStr(key string, value string, time int) error
	Incr(key string, time int, timeReset bool) error
	Exists(key string) bool
	GetKeyExpireTime(key string) int64
	GetInt(key string) (int64, error)
	GetStr(key string) (string, error)
	Del(key string) error
}

//NewCache ...
func NewCache() Cache {
	return &cache{RedisCache}
}
func (d *cache) SetStr(key string, value string, time int) error {
	r, err := db.GetRedisPoll(d.db)
	if err != nil {
		return err
	}
	conn := r.Get()
	defer conn.Close()
	if err != nil {
		return err
	}
	_, err = conn.Do("SET", key, value)
	if err != nil {
		return err
	}
	if time > 0 {
		_, err = conn.Do("EXPIRE", key, time) // EXPIRE 毫秒
		if err != nil {
			return err
		}
	}
	return nil
}

//Incr 递增一个数字 默认从0开始
func (d *cache) Incr(key string, time int, timeReset bool) error {
	r, err := db.GetRedisPoll(d.db)
	if err != nil {
		return err
	}
	conn := r.Get()
	defer conn.Close()

	n, err := redis.Int64(conn.Do("INCR", key))
	if err != nil {
		return err
	}
	if n <= 1 {
		timeReset = true
	}
	if timeReset {
		if time > 0 {
			_, err = conn.Do("EXPIRE", key, time) //EXPIRE秒 PEXPIRE 毫秒
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Exists check a key
func (d *cache) Exists(key string) bool {
	r, err := db.GetRedisPoll(d.db)
	if err != nil {
		return false
	}
	conn := r.Get()
	defer conn.Close()

	exists, err := redis.Bool(conn.Do("EXISTS", key))
	if err != nil {
		return false
	}
	return exists
}

//GetKeyExpireTime 获取key的剩余过期时间 单位秒 如果key是永久的，返回-1；如果key不存在或者已过期，返回-2。
func (d *cache) GetKeyExpireTime(key string) int64 {
	r, err := db.GetRedisPoll(d.db)
	if err != nil {
		return -2
	}
	conn := r.Get()
	defer conn.Close()

	t, err := redis.Int64(conn.Do("TTL", key)) // 单位是秒
	if err != nil {
		return -2
	}
	return t
}

//GetInt 获取某个key 的int值
func (d *cache) GetInt(key string) (int64, error) {
	r, err := db.GetRedisPoll(d.db)
	if err != nil {
		return 0, err
	}
	conn := r.Get()
	defer conn.Close()

	n, err := redis.Int64(conn.Do("GET", key))
	if err != nil {
		return 0, err
	}
	return n, nil
}

//GetStr 获取某个key 的str值
func (d *cache) GetStr(key string) (string, error) {
	r, err := db.GetRedisPoll(d.db)
	if err != nil {
		return "", err
	}
	conn := r.Get()
	defer conn.Close()

	n, err := redis.String(conn.Do("GET", key))
	if err != nil {
		return "", err
	}
	return n, nil
}

//Del 某个key
func (d *cache) Del(key string) error {
	r, err := db.GetRedisPoll(d.db)
	if err != nil {
		return err
	}
	conn := r.Get()
	defer conn.Close()

	_, err = conn.Do("DEL", key)
	if err != nil {
		return err
	}
	return nil
}
