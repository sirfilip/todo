package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/boltdb/bolt"
	"github.com/olekukonko/tablewriter"
)

type status int

func (s status) String() string {
	switch s {
	case statusPending:
		return "Pending"
	case statusDone:
		return "Done"
	case statusCanceled:
		return "Canceled"
	default:
		return fmt.Sprintf("Unknown status: %v", int(s))
	}
}

const (
	statusPending  status = 0
	statusDone     status = 1
	statusCanceled status = 2
)

type todo struct {
	Task    string
	Created time.Time
	Status  status
}

func (t todo) String() string {
	return fmt.Sprintf("%s | %s | %s", t.Created.Format(time.UnixDate), t.Task, t.Status)
}

type todos []todo

func (tc todos) filter(status status) todos {
	var res []todo
	for _, todo := range tc {
		if todo.Status == status {
			res = append(res, todo)
		}
	}
	return todos(res)
}

func (tc todos) render(w io.Writer) {
	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Created", "Task", "Status"})
	for _, todo := range tc {
		table.Append([]string{todo.Created.Format(time.Stamp), todo.Task, todo.Status.String()})
	}
	table.Render()
}

func main() {
	err := run()
	if err != nil {
		panic(err)
	}
}

func run() error {
	dbName := flag.String("d", "todos", "DB name")
	cmd := flag.String("c", "pending", "Execute command")
	task := flag.String("t", "", "Task name")

	flag.Parse()

	db, err := bolt.Open(fmt.Sprintf("%s/.%s", os.Getenv("HOME"), *dbName), 0600, nil)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("default"))
		return err
	}); err != nil {
		return err
	}

	switch *cmd {
	case "all":
		return showAllTasks(os.Stdout, db)
	case "pending":
		return showTasks(os.Stdout, db, statusPending)
	case "completed":
		return showTasks(os.Stdout, db, statusDone)
	case "create":
		return createTask(db, *task)
	case "close":
		return closeTask(db, *task)
	default:
		return fmt.Errorf("Unknown command %q", *cmd)
	}
}

func load(db *bolt.DB) (todos, error) {
	var res []todo
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("default"))
		data := b.Get([]byte("todos"))
		if data == nil {
			res = []todo{}
			return nil
		}
		return json.Unmarshal(data, &res)
	})
	return todos(res), err
}

func store(db *bolt.DB, tc todos) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("default"))
		data, err := json.Marshal(tc)
		if err != nil {
			return err
		}
		return b.Put([]byte("todos"), data)
	})
}

func showAllTasks(w io.Writer, db *bolt.DB) error {
	todos, err := load(db)
	if err != nil {
		return fmt.Errorf("showAllTasks: %w", err)
	}
	todos.render(w)
	return nil
}

func showTasks(w io.Writer, db *bolt.DB, status status) error {
	todos, err := load(db)
	if err != nil {
		return fmt.Errorf("showTasks: %w", err)
	}
	todos.filter(status).render(w)
	return nil

}

func createTask(db *bolt.DB, task string) error {
	todos, err := load(db)
	if err != nil {
		return fmt.Errorf("createTask: %w", err)
	}
	todos = append(todos, todo{Task: task, Created: time.Now(), Status: statusPending})
	return store(db, todos)
}

func closeTask(db *bolt.DB, task string) error {
	var updated []todo
	todos, err := load(db)
	if err != nil {
		return err
	}
	for _, t := range todos {
		if t.Task == task {
			updated = append(updated, todo{Task: task, Created: t.Created, Status: statusDone})
			continue
		}
		updated = append(updated, t)
	}
	return store(db, updated)

}
