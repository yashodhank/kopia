package snapshot

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/kopia/kopia/fs"
	"github.com/kopia/kopia/internal/dir"
	"github.com/kopia/kopia/object"
	"github.com/kopia/kopia/repo"
)

type repositoryEntry struct {
	parent   fs.Directory
	metadata *dir.Entry
	repo     *repo.Repository
}

func (e *repositoryEntry) Parent() fs.Directory {
	return e.parent
}

func (e *repositoryEntry) Metadata() *fs.EntryMetadata {
	return &e.metadata.EntryMetadata
}

func (e *repositoryEntry) ObjectID() object.ID {
	return e.metadata.ObjectID
}

type repositoryDirectory struct {
	repositoryEntry
}

type repositoryFile struct {
	repositoryEntry
}

type repositorySymlink struct {
	repositoryEntry
}

func (rd *repositoryDirectory) Readdir(ctx context.Context) (fs.Entries, error) {
	r, err := rd.repo.Objects.Open(ctx, rd.metadata.ObjectID)
	if err != nil {
		return nil, err
	}
	defer r.Close() //nolint:errcheck

	metadata, _, err := dir.ReadEntries(r)
	if err != nil {
		return nil, err
	}

	entries := make(fs.Entries, len(metadata))
	for i, m := range metadata {
		entries[i] = newRepoEntry(rd.repo, m, rd)
	}

	return entries, nil
}

func (rf *repositoryFile) Open(ctx context.Context) (fs.Reader, error) {
	r, err := rf.repo.Objects.Open(ctx, rf.metadata.ObjectID)
	if err != nil {
		return nil, err
	}

	return withMetadata(r, &rf.metadata.EntryMetadata), nil
}

func (rsl *repositorySymlink) Readlink(ctx context.Context) (string, error) {
	r, err := rsl.repo.Objects.Open(ctx, rsl.metadata.ObjectID)
	if err != nil {
		return "", err
	}

	defer r.Close() //nolint:errcheck
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func newRepoEntry(r *repo.Repository, md *dir.Entry, parent fs.Directory) fs.Entry {
	re := repositoryEntry{
		metadata: md,
		parent:   parent,
		repo:     r,
	}
	switch md.Type {
	case fs.EntryTypeDirectory:
		return fs.Directory(&repositoryDirectory{re})

	case fs.EntryTypeSymlink:
		return fs.Symlink(&repositorySymlink{re})

	case fs.EntryTypeFile:
		return fs.File(&repositoryFile{re})

	default:
		panic(fmt.Sprintf("not supported entry metadata type: %v", md.Type))
	}
}

type entryMetadataReadCloser struct {
	object.Reader
	metadata *fs.EntryMetadata
}

func (emrc *entryMetadataReadCloser) EntryMetadata() (*fs.EntryMetadata, error) {
	return emrc.metadata, nil
}

func withMetadata(r object.Reader, md *fs.EntryMetadata) fs.Reader {
	return &entryMetadataReadCloser{r, md}
}

// DirectoryEntry returns fs.Directory based on repository object with the specified ID.
// The existence or validity of the directory object is not validated until its contents are read.
func (m *Manager) DirectoryEntry(objectID object.ID) fs.Directory {
	d := newRepoEntry(m.repository, &dir.Entry{
		EntryMetadata: fs.EntryMetadata{
			Name:        "/",
			Permissions: 0555,
			Type:        fs.EntryTypeDirectory,
		},
		ObjectID: objectID,
	}, nil)

	return d.(fs.Directory)
}

var _ fs.Directory = &repositoryDirectory{}
var _ fs.File = &repositoryFile{}
var _ fs.Symlink = &repositorySymlink{}
