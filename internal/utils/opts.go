package utils

import (
	"flag"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type OptsType struct {
	ServerPort              int
	DbHost                  string
	DbUser                  string
	DbPassword              string
	DbName                  string
	DbPort                  string
	BitcoinTestHost         string
	BitcoinTestUser         string
	BitcoinTestPass         string
	BitcoinMainHost         string
	BitcoinMainUser         string
	BitcoinMainPass         string
	ProxyBaseUrl            string
	BackendBaseUrl          string
	TestWalletPassphrase    string
	MainWalletPassphrase    string
	TestChangeAddress       string
	MainChangeAddress       string
	ForwardAmountPercentage int
	FallbackFee             float64
	MinimumConfirmations    int
}

var (
	Opts *OptsType
)

func NewOpts() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Could not find env file [%v], using defaults", err)
	}

	o := &OptsType{}
	flag.IntVar(&o.ServerPort, "SERVER_PORT", lookupEnvInt("SERVER_PORT", 9001), "Server PORT")
	flag.StringVar(&o.DbHost, "DB_HOST", lookupEnv("DB_HOST", "localhost"), "Database Host")
	flag.StringVar(&o.DbUser, "DB_USER", lookupEnv("DB_USER", "postgres"), "Database User")
	flag.StringVar(&o.DbPassword, "DB_PASSWORD", lookupEnv("DB_PASSWORD"), "Database Password")
	flag.StringVar(&o.DbName, "DB_NAME", lookupEnv("DB_NAME", "bitcoin"), "Database Name")
	flag.StringVar(&o.DbPort, "DB_PORT", lookupEnv("DB_PORT", "5434"), "Database Port")
	flag.StringVar(&o.BitcoinTestHost, "BITCOIN_TEST_HOST", lookupEnv("BITCOIN_TEST_HOST", "localhost:8333"), "Bitcoin Host")
	flag.StringVar(&o.BitcoinTestUser, "BITCOIN_TEST_USER", lookupEnv("BITCOIN_TEST_USER", "user"), "Bitcoin User")
	flag.StringVar(&o.BitcoinTestPass, "BITCOIN_TEST_PASS", lookupEnv("BITCOIN_TEST_PASS"), "Bitcoin Password")
	flag.StringVar(&o.BitcoinMainHost, "BITCOIN_MAIN_HOST", lookupEnv("BITCOIN_MAIN_HOST", "localhost:8333"), "Bitcoin Host")
	flag.StringVar(&o.BitcoinMainUser, "BITCOIN_MAIN_USER", lookupEnv("BITCOIN_MAIN_USER"), "Bitcoin User")
	flag.StringVar(&o.BitcoinMainPass, "BITCOIN_MAIN_PASS", lookupEnv("BITCOIN_MAIN_PASS"), "Bitcoin Password")
	flag.StringVar(&o.ProxyBaseUrl, "PROXY_BASE_URL", lookupEnv("PROXY_BASE_URL", "http://localhost:8001/api"), "Proxy base url")
	flag.StringVar(&o.BackendBaseUrl, "BACKEND_BASE_URL", lookupEnv("BACKEND_BASE_URL", "http://localhost:8000/api/internal"), "Backend base url")
	flag.StringVar(&o.TestWalletPassphrase, "TEST_WALLET_PASSPHRASE", lookupEnv("TEST_WALLET_PASSPHRASE"), "TEST WALLET PASSPHRASE")
	flag.StringVar(&o.MainWalletPassphrase, "MAIN_WALLET_PASSPHRASE", lookupEnv("MAIN_WALLET_PASSPHRASE"), "MAIN WALLET PASSPHRASE")
	flag.StringVar(&o.TestChangeAddress, "TEST_CHANGE_ADDRESS", lookupEnv("TEST_CHANGE_ADDRESS"), "TEST_CHANGE_ADDRESS")
	flag.StringVar(&o.MainChangeAddress, "MAIN_CHANGE_ADDRESS", lookupEnv("MAIN_CHANGE_ADDRESS"), "MAIN_CHANGE_ADDRESS")
	flag.IntVar(&o.ForwardAmountPercentage, "FORWARD_AMOUNT_PERCENTAGE", lookupEnvInt("FORWARD_AMOUNT_PERCENTAGE", 99), "FORWARD_AMOUNT_PERCENTAGE")
	flag.Float64Var(&o.FallbackFee, "FALLBACK_FEE", lookupEnvFloat64("FALLBACK_FEE", 0.00002986), "FALLBACK_FEE")
	flag.IntVar(&o.MinimumConfirmations, "MINIMUM_CONFIRMATIONS", lookupEnvInt("MINIMUM_CONFIRMATIONS", 6), "MINIMUM_CONFIRMATIONS")

	Opts = o
}

func lookupEnv(key string, defaultValues ...string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	for _, v := range defaultValues {
		if v != "" {
			return v
		}
	}
	return ""
}

func lookupEnvInt(key string, defaultValues ...int) int {
	if val, ok := os.LookupEnv(key); ok {
		v, err := strconv.Atoi(val)
		if err != nil {
			log.Printf("LookupEnvInt[%s]: %v", key, err)
			return 0
		}
		return v
	}
	for _, v := range defaultValues {
		if v != 0 {
			return v
		}
	}
	return 0
}

func lookupEnvFloat64(key string, defaultValues ...float64) float64 {
	if val, ok := os.LookupEnv(key); ok {
		v, err := strconv.ParseFloat(val, 64)
		if err != nil {
			log.Printf("LookupEnvParse[%s]: %v", key, err)
			return 0.0
		}
		return v
	}
	for _, v := range defaultValues {
		if v != 0.0 {
			return v
		}
	}
	return 0.0
}
