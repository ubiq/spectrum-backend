package util

import (
	"math/big"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/ubiq/go-ubiq/common/hexutil"
)

func DecodeHex(str string) uint64 {
	if len(str) < 2 {
		//log.Errorf("Invalid string: %v", str)
		return 0
	}
	if str == "0x0" || len(str) == 0 {
		return 0
	}

	if str[:2] == "0x" {
		str = str[2:]
	}

	i, err := strconv.ParseUint(str, 16, 64)

	if err != nil {
		log.Errorf("Couldn't decode hex (%v): %v", str, err)
		return 0
	}

	return i
}

func DecodeValueHex(val string) string {
	x, err := hexutil.DecodeBig(val)

	if err != nil {
		log.Errorf("ErrorDecodeValueHex (%v): %v", val, err)
	}
	return x.String()
}

func InputParamsToAddress(str string) string {
	return "0x" + strings.ToLower(str[24:])
}

func CaculateBlockReward(height uint64, uncleNo int) *big.Int {

	baseReward := baseBlockReward(height)

	uncleRewards := big.NewInt(0)

	if uncleNo > 0 {
		uncleRewards = uncleRewards.Div(baseReward, big.NewInt(int64(32*uncleNo)))
	}

	baseReward = baseReward.Add(baseReward, uncleRewards)
	return baseReward
}

func CaculateUncleReward(height uint64, uncleHeight uint64) *big.Int {
	baseReward := baseBlockReward(height)

	uncleRewards := big.NewInt(0)

	uncleRewards.Mul(big.NewInt(int64((uncleHeight+2)-height)), baseReward)
	uncleRewards.Div(uncleRewards, big.NewInt(2))

	if uncleRewards.Cmp(big.NewInt(0)) == -1 {
		return big.NewInt(0)
	}
	return uncleRewards
}

func baseBlockReward(height uint64) *big.Int {
	if height > 2508545 {
		return big.NewInt(1000000000000000000)
	} else if height > 2150181 {
		return big.NewInt(2000000000000000000)
	} else if height > 1791818 {
		return big.NewInt(3000000000000000000)
	} else if height > 1433454 {
		return big.NewInt(4000000000000000000)
	} else if height > 1075090 {
		return big.NewInt(5000000000000000000)
	} else if height > 716727 {
		return big.NewInt(6000000000000000000)
	} else if height > 358363 {
		return big.NewInt(7000000000000000000)
	} else if height > 0 {
		return big.NewInt(8000000000000000000)
	} else {
		// genesis
		return big.NewInt(0)
	}
}
