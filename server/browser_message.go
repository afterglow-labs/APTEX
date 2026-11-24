package server

import "github.com/afterglow-labs/APTEX/locale"

func trMsg(message string) string { return locale.Text(locale.ServerDefault(), message) }
