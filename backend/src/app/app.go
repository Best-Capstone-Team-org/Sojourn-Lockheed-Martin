package app

import (
	"log"
	"os"
)

var ErrorLogger *log.Logger
var LoaderLogger *log.Logger
var ServerLogger *log.Logger

func InitLoggers() {
	ErrorLogger = log.New(os.Stderr, "[ERROR] ", log.Ldate|log.Ltime)
	LoaderLogger = log.New(os.Stdout, "[LOADER] ", log.Ldate|log.Ltime)
	ServerLogger = log.New(os.Stdout, "[SERVER] ", log.Ldate|log.Ltime)
}
