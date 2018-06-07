package main

import "github.com/kelseyhightower/envconfig"

type ConfigurationSpec struct {
	Port                     int    `default:"8004"`
	CourseProgressServiceUrl string `default:"http://127.0.0.1:8003" envconfig:"PROGRESS_SERVICE_URL"`
	StatsServiceUrl string `default:"http://127.0.0.1:8002" envconfig:"STATS_SERVICE_URL"`
}

var config ConfigurationSpec

func initConfig() {
	envconfig.MustProcess("achievement", &config)
}
