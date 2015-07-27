package main

import (
	"errors"
	"io/ioutil"
	"os/user"
	"path"

	"github.com/nyxtom/broadcast/server"
)

type TermBackend struct {
	server.Backend

	homeDir    string
	resumeText []byte
	app        *server.BroadcastServer
}

func (t *TermBackend) ShowResume(data interface{}, client server.ProtocolClient) error {
	client.WriteBytes(t.resumeText)
	client.Flush()
	return nil
}

func (t *TermBackend) CatFile(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([][]byte)
	if len(d) > 0 {
		fileName := string(d[0])
		content, err := ioutil.ReadFile(path.Join(t.homeDir, fileName))
		if err != nil {
			client.WriteError(err)
		} else {
			client.WriteBytes(content)
		}
		client.Flush()
	} else {
		client.WriteError(errors.New("cat takes at least 1 parameter (cat filename)"))
		client.Flush()
	}

	return nil
}

func (t *TermBackend) ListFiles(data interface{}, client server.ProtocolClient) error {
	files, err := ioutil.ReadDir(t.homeDir)

	filenames := []interface{}{}
	if err != nil {
		client.WriteError(err)
		client.Flush()
	} else {
		for _, f := range files {
			if !f.IsDir() {
				filenames = append(filenames, f.Name())
			}
		}
		client.WriteArray(filenames)
		client.Flush()
	}
	return nil
}

func (t *TermBackend) EditFile(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([][]byte)
	if len(d) > 0 {
		fileName := string(d[0])
		content, err := ioutil.ReadFile(path.Join(t.homeDir, fileName))
		if err == nil {
			fileMap := make(map[string]string)
			fileMap["filename"] = fileName
			fileMap["contents"] = string(content)
			client.WriteJson(fileMap)
			client.Flush()
		} else {
			fileMap := make(map[string]string)
			fileMap["filename"] = fileName
			fileMap["contents"] = ""
			client.WriteJson(fileMap)
			client.Flush()
		}
	}

	return nil
}

func (t *TermBackend) SaveFile(data interface{}, client server.ProtocolClient) error {
	d, _ := data.([][]byte)
	if len(d) >= 2 {
		fileName := string(d[0])
		content := d[1]
		err := ioutil.WriteFile(fileName, content, 0644)
		if err != nil {
			client.WriteError(err)
			client.Flush()
		} else {
			client.WriteString("saved " + fileName + " successfully")
			client.Flush()
		}
	} else {
		client.WriteError(errors.New("save takes at least 2 parameters (save filename filecontents...)"))
		client.Flush()
	}

	return nil
}

func RegisterTermBackend(app *server.BroadcastServer, homeDir string) (server.Backend, error) {
	backend := new(TermBackend)
	backend.app = app

	// locate the resume content from the home directory
	if homeDir == "" {
		usr, _ := user.Current()
		homeDir = usr.HomeDir
	}
	backend.homeDir = homeDir

	app.RegisterCommand(server.Command{"cat", "Concatenate the contents of a file", "", false}, backend.CatFile)
	app.RegisterCommand(server.Command{"ls", "Lists the files in the directory", "", false}, backend.ListFiles)
	app.RegisterCommand(server.Command{"dir", "Lists the files in the directory", "", false}, backend.ListFiles)
	app.RegisterCommand(server.Command{"edit", "Edit the contents of a file", "", false}, backend.EditFile)
	app.RegisterCommand(server.Command{"save", "Saves the contents of a file", "", false}, backend.SaveFile)
	return backend, nil
}

func (b *TermBackend) Load() error {
	return nil
}

func (b *TermBackend) Unload() error {
	return nil
}
