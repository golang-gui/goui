package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-gui/goui/cmd/goui/internal/applecerts"
)

func runMacOSDistribution(ctx context.Context, command string, args []string, out, errOut io.Writer) error {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(errOut)
	configPath := flags.String("config", "goui.toml", "configuration file")
	targetName := flags.String("target", "darwin/arm64", "macOS target (darwin/arm64)")
	app := flags.String("app", "", "existing .app directory (default: package output)")
	var output, submission string
	var wait, history bool
	var timeout time.Duration
	if command != "staple" {
		flags.StringVar(&output, "o", "", "archive or notarization receipt output, relative to configuration")
	}
	if command == "notarize" {
		flags.StringVar(&submission, "submission", "", "query an existing submission ID without uploading again")
		flags.BoolVar(&wait, "wait", false, "wait for a terminal notarization status")
		flags.BoolVar(&history, "history", false, "list recent notarization submissions without uploading")
	}
	if command != "archive" {
		flags.DurationVar(&timeout, "timeout", 10*time.Minute, "maximum operation duration")
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	if *targetName != "darwin/arm64" {
		return fmt.Errorf("%s supports only darwin/arm64", command)
	}
	if command != "archive" {
		if timeout <= 0 {
			return fmt.Errorf("timeout must be positive")
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	c, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	if *app == "" && submission == "" && !history {
		*app = filepath.Join("dist", "darwin-arm64", c.Name+".app")
	}
	appPath := ""
	if *app != "" {
		appPath = c.path(*app)
	}
	if command == "archive" {
		if output == "" {
			output = filepath.Join("dist", "darwin-arm64", c.Name+".zip")
		}
		if err = archiveMacOS(ctx, appPath, c.path(output)); err != nil {
			return err
		}
		fmt.Fprintln(out, c.path(output))
		return nil
	}
	roots, err := applecerts.Roots()
	if err != nil {
		return err
	}
	if command == "staple" {
		if err = stapleMacOS(ctx, appPath, roots); err != nil {
			return err
		}
		fmt.Fprintln(out, appPath)
		return nil
	}
	if history && (*app != "" || submission != "" || wait || output != "") {
		return fmt.Errorf("notarize -history cannot be combined with app, submission, wait or output")
	}
	if submission != "" && !notaryIDPattern.MatchString(submission) {
		return fmt.Errorf("invalid notarization submission ID")
	}
	if submission != "" && *app != "" {
		return fmt.Errorf("notarize -submission cannot be combined with -app")
	}
	client, err := newNotaryClient(c)
	if err != nil {
		return err
	}
	if history {
		var reply json.RawMessage
		if err = client.request(ctx, http.MethodGet, "/submissions", nil, &reply); err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, string(reply))
		return err
	}
	receipt := notaryReceipt{Format: 1, ID: submission, Status: "Prepared"}
	if output == "" {
		name := c.Name
		if submission != "" {
			name = submission
		}
		output = filepath.Join("dist", "darwin-arm64", name+".notary.json")
	}
	receiptPath := c.path(output)
	if submission != "" {
		if raw, e := os.ReadFile(receiptPath); e == nil {
			if e = json.Unmarshal(raw, &receipt); e != nil {
				return e
			}
			if receipt.Format != 1 || receipt.ID != submission {
				return fmt.Errorf("receipt does not match requested submission")
			}
		} else if !os.IsNotExist(e) {
			return e
		} else if err = saveNotaryReceipt(receiptPath, &receipt, true); err != nil {
			return err
		}
	} else {
		hash, err := verifyMacOSBundle(appPath, roots)
		if err != nil {
			return err
		}
		work, err := os.MkdirTemp("", "goui-notary-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(work)
		zipPath := filepath.Join(work, filepath.Base(appPath)+".zip")
		if err = archiveMacOS(ctx, appPath, zipPath); err != nil {
			return err
		}
		current, err := verifyMacOSBundle(appPath, roots)
		if err != nil {
			return err
		}
		if current != hash {
			return fmt.Errorf("app changed while creating notarization archive")
		}
		file, err := os.Open(zipPath)
		if err != nil {
			return err
		}
		digest := sha256.New()
		_, err = io.Copy(digest, file)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		receipt.Name = filepath.Base(zipPath)
		receipt.SHA256 = hex.EncodeToString(digest.Sum(nil))
		receipt.CDHash = hex.EncodeToString(hash[:])
		if err = saveNotaryReceipt(receiptPath, &receipt, true); err != nil {
			return fmt.Errorf("reserve new notarization receipt: %w", err)
		}
		reply, err := client.submit(ctx, receipt.Name, receipt.SHA256)
		if err != nil {
			receipt.Status = "Submission result unknown"
			saveErr := saveNotaryReceipt(receiptPath, &receipt, false)
			return errors.Join(fmt.Errorf("submit notarization: %w; receipt retained; inspect -history before submitting again", err), saveErr)
		}
		receipt.ID = reply.Data.ID
		receipt.Status = "Uploading"
		if err = saveNotaryReceipt(receiptPath, &receipt, false); err != nil {
			return fmt.Errorf("save submission %s: %w", receipt.ID, err)
		}
		fmt.Fprintln(errOut, "Notary submission:", receipt.ID)
		if err = client.upload(ctx, reply, zipPath, receipt.SHA256); err != nil {
			receipt.Status = "Upload failed"
			saveErr := saveNotaryReceipt(receiptPath, &receipt, false)
			return errors.Join(fmt.Errorf("upload submission %s: %w", receipt.ID, err), saveErr)
		}
		receipt.Uploaded = true
		receipt.Status = "In Progress"
		if err = saveNotaryReceipt(receiptPath, &receipt, false); err != nil {
			return err
		}
	}
	if err = client.poll(ctx, &receipt, wait, func() error { return saveNotaryReceipt(receiptPath, &receipt, false) }); err != nil {
		return fmt.Errorf("submission %s (receipt %s): %w", receipt.ID, receiptPath, err)
	}
	return json.NewEncoder(out).Encode(receipt)
}
