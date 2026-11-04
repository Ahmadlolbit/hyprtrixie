package main

import (
	"cmp"
	"fmt"
	"os"
	"strings"

	"github.com/Ahmadlolbit/hyprtrixie/internal/databases"
	"github.com/Ahmadlolbit/hyprtrixie/internal/env"
)

const dbUsage = "usage: hyprtrixie db [list | start NAME | stop NAME | remove NAME], where NAME is postgres, mysql or redis"

func dbList() int {
	text := databases.List()
	if err := databases.Engine(); err != nil {
		text += "\n" + err.Error() + "\n"
	}
	return say(text)
}

func dbAction(action string, database databases.Database) int {
	var err error
	switch action {
	case "start":
		var address string
		if address, err = database.Start(os.Stdout); err == nil {
			return say(fmt.Sprintf("%s is running on localhost: %s\n", database.Title, address))
		}
	case "stop":
		if err = database.Stop(os.Stdout); err == nil {
			return say(database.Title + " stopped. Its data stays for the next start; remove deletes it.\n")
		}
	case "remove":
		if err = database.Remove(os.Stdout); err == nil {
			return say(database.Title + " and its data are gone.\n")
		}
	default:
		return fail(dbUsage)
	}
	return fail("%v", err)
}

func dbCommand() int {
	if os.Geteuid() == 0 {
		return fail("run it as yourself, without sudo: hyprtrixie db %s", strings.Join(os.Args[2:], " "))
	}
	action := cmp.Or(env.Arg(2), "list")
	if action == "list" {
		return dbList()
	}
	database, found := databases.Find(env.Arg(3))
	if !found {
		return fail(dbUsage)
	}
	return dbAction(action, database)
}
