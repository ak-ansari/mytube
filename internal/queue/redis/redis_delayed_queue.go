package redisQueue

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Delayed messages live in the sorted set "<queue>:delayed", scored by the unix-ms time they become due.
// Each member is "<uuid>|<payload>" so identical payloads scheduled twice don't collapse into one.
func delayedKey(queueName string) string { return queueName + ":delayed" }

const memberSep = "|"

// promoteScript atomically moves due members to the list, so two workers never promote the same message.
var promoteScript = redis.NewScript(`
local due = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1], 'LIMIT', 0, tonumber(ARGV[2]))
for _, m in ipairs(due) do
  redis.call('ZREM', KEYS[1], m)
  local sep = string.find(m, '|', 1, true)
  redis.call('RPUSH', KEYS[2], string.sub(m, sep + 1))
end
return #due
`)

func (r *redisQueue) EnqueueAfter(ctx context.Context, queueName string, payload []byte, delay time.Duration) error {
	if delay <= 0 {
		return r.Enqueue(ctx, queueName, payload)
	}
	member := uuid.NewString() + memberSep + string(payload)
	due := time.Now().Add(delay).UnixMilli()
	return r.client.ZAdd(ctx, delayedKey(queueName), redis.Z{Score: float64(due), Member: member}).Err()
}

func (r *redisQueue) PromoteDue(ctx context.Context, queueName string, now time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	n, err := promoteScript.Run(ctx, r.client, []string{delayedKey(queueName), queueName}, now.UnixMilli(), limit).Int()
	if err != nil {
		return 0, err
	}
	return n, nil
}
