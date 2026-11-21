// support-tickets is the repo owner's tool for answering support tickets while
// there is no admin role. It connects with the owner's DATABASE_URL only and
// needs no Firebase credentials. Run it only against the intended database.
//
//	list   [--status S] [--id ID] [--limit N]                      read tickets (JSON)
//	status --id ID --status S --confirm                            change status
//	reply  --id ID --message T|--message-file F [--status S] [--replace] --confirm
//
// Status changes follow open->in_progress|resolved, in_progress->resolved and
// resolved->in_progress. Nothing acknowledges a ticket automatically and a
// reply never changes points, XP, vouchers, or any other balance. The user
// sees the reply the next time they open their ticket; nothing is e-mailed
// or pushed. Only the latest reply is stored; replacing needs --replace.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/fahrypratama17/EcoSnap-BE/internal/repository/postgres"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

const usage = "gunakan: support-tickets list|status|reply (lihat dokumentasi); DATABASE_URL wajib; perubahan wajib --confirm"

// maxMessageFile bounds --message-file; replies are at most 4000 characters.
const maxMessageFile = 32 * 1024

type opener func(ctx context.Context, databaseURL string) (service.SupportOwnerStore, func(), error)

func openPostgres(ctx context.Context, databaseURL string) (service.SupportOwnerStore, func(), error) {
	pool, err := postgres.NewPool(ctx, databaseURL)
	if err != nil {
		return nil, nil, err
	}
	return postgres.NewSupportRepository(pool), pool.Close, nil
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Getenv, os.Stdout, openPostgres); err != nil {
		fmt.Fprintln(os.Stderr, err)
		cancel()
		os.Exit(1)
	}
}

var connectionString = regexp.MustCompile(`(?i)postgres(ql)?://\S+`)

// redact removes the connection string and its password from an error
// message; driver errors can echo them.
func redact(err error, databaseURL string) string {
	msg := err.Error()
	if databaseURL != "" {
		msg = strings.ReplaceAll(msg, databaseURL, "[DATABASE_URL]")
		if u, perr := url.Parse(databaseURL); perr == nil && u.User != nil {
			if pw, ok := u.User.Password(); ok && pw != "" {
				msg = strings.ReplaceAll(msg, pw, "[redacted]")
			}
		}
	}
	return connectionString.ReplaceAllString(msg, "[DATABASE_URL]")
}

func run(ctx context.Context, args []string, getenv func(string) string, out io.Writer, open opener) error {
	if len(args) < 1 {
		return errors.New(usage)
	}
	cmd := args[0]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	id := fs.String("id", "", "ID tiket")
	status := fs.String("status", "", "open|in_progress|resolved")
	message := fs.String("message", "", "teks balasan (reply)")
	messageFile := fs.String("message-file", "", "file teks balasan (reply)")
	limit := fs.Int("limit", 20, "jumlah maksimum tiket (list)")
	replace := fs.Bool("replace", false, "ganti balasan yang sudah ada (reply)")
	confirm := fs.Bool("confirm", false, "konfirmasi perubahan")
	if cmd != "list" && cmd != "status" && cmd != "reply" {
		return errors.New(usage)
	}
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
		return errors.New(usage)
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	var statusArg, responseArg *string
	if set["status"] {
		statusArg = status
	}
	switch cmd {
	case "list":
		if set["message"] || set["message-file"] || set["replace"] || *confirm {
			return errors.New("list hanya menerima --status, --id, --limit")
		}
	case "status":
		if !*confirm {
			return errors.New("--confirm wajib untuk mengubah status")
		}
		if *id == "" || !set["status"] || set["message"] || set["message-file"] || set["replace"] || set["limit"] {
			return errors.New("status memerlukan --id dan --status saja")
		}
	case "reply":
		if !*confirm {
			return errors.New("--confirm wajib untuk membalas")
		}
		if *id == "" || set["limit"] || set["message"] == set["message-file"] {
			return errors.New("reply memerlukan --id dan tepat satu dari --message atau --message-file")
		}
		text := *message
		if set["message-file"] {
			f, err := os.Open(*messageFile)
			if err != nil {
				return errors.New("file balasan tidak dapat dibuka")
			}
			raw, err := io.ReadAll(io.LimitReader(f, maxMessageFile+1))
			f.Close()
			if err != nil || len(raw) > maxMessageFile {
				return errors.New("file balasan tidak dapat dibaca atau terlalu besar")
			}
			text = string(raw)
		}
		responseArg = &text
	}
	databaseURL := getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL wajib diisi")
	}
	store, closeStore, err := open(ctx, databaseURL)
	if err != nil {
		return errors.New("koneksi database gagal")
	}
	defer closeStore()
	owner := service.NewSupportOwnerService(store)

	var result any
	switch cmd {
	case "list":
		result, err = owner.List(ctx, *id, *status, *limit)
	default:
		result, err = owner.Update(ctx, *id, statusArg, responseArg, *replace)
	}
	if err != nil {
		return errors.New(redact(err, databaseURL))
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]any{"data": result})
}
