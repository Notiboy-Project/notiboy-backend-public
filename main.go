package main

import (
	"time"
	_ "time/tzdata"

	"notiboy/app"
)

func main() {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		panic(err)
	}

	time.Local = loc
	app.Run()
}
