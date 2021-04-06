package service

import (
	"crypto/rand"
	guuid "github.com/google/uuid"
	"math"
	"math/big"
	"sync/atomic"
	"time"
)

type UUID string

func (id UUID) Short() string {
	var idStr = string(id)
	if len(idStr) > 8 {
		return idStr[:8]
	} else {
		return idStr
	}
}

func RandomUUID() UUID {
	return UUID(guuid.New().String())
}

func RandomString() string {
	return guuid.New().String()
}

func MakeRandomTimer(count int) *time.Timer {
	var sleepSec float64
	if count < 32 {
		sleepSec = math.Pow(2, float64(MakeRandomInt(count)))
		if sleepSec > 120 {
			sleepSec = 120
		}
	} else {
		sleepSec = 120
	}
	timer := time.NewTimer(time.Duration(sleepSec) * time.Second)
	return timer
}

func MakeRandomInt(max int) int {
	if i, err := rand.Int(rand.Reader, big.NewInt(int64(max))); err != nil {
		return 1
	} else {
		return int(i.Int64())
	}
}

type generator struct {
	number uint64
}

var g *generator

func GetSeqNo() uint64 {
	return atomic.AddUint64(&g.number, 1)
}

func init() {
	g = new(generator)
}
