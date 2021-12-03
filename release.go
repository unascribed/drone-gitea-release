package main

import (
	"fmt"
	"os"
	"path"

	"code.gitea.io/sdk/gitea"
)

// Release holds ties the drone env data and gitea client together.
type releaseClient struct {
	*gitea.Client
	Owner      string
	Repo       string
	Tag        string
	Draft      bool
	Prerelease bool
	FileExists string
	Title      string
	Note       string
	AllowEdit  bool
}

func (rc *releaseClient) buildRelease() (*gitea.Release, error) {
	// first attempt to get a release by that tag
	release, err := rc.getRelease()

	if err == nil && release != nil {
		if rc.AllowEdit {
			release, err = rc.editRelease(release.ID)

			if err != nil {
				return nil, err
			}
		}

		return release, nil
	}

	if err != nil && release == nil {
		fmt.Println(err)
	}

	// if no release was found by that tag, create a new one
	release, err = rc.newRelease()

	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve or create a release: %s", err)
	}

	return release, nil
}

func (rc *releaseClient) getRelease() (*gitea.Release, error) {
	releases, _, err := rc.Client.ListReleases(rc.Owner, rc.Repo, gitea.ListReleasesOptions{})
	if err != nil {
		return nil, err
	}

	for _, release := range releases {
		if release.TagName == rc.Tag {
			fmt.Printf("Successfully retrieved %s release\n", rc.Tag)
			return release, nil
		}
	}
	return nil, fmt.Errorf("Release %s not found", rc.Tag)
}

func (rc *releaseClient) newRelease() (*gitea.Release, error) {
	r := gitea.CreateReleaseOption{
		TagName:      rc.Tag,
		IsDraft:      rc.Draft,
		IsPrerelease: rc.Prerelease,
		Title:        rc.Title,
		Note:         rc.Note,
	}

	release, _, err := rc.Client.CreateRelease(rc.Owner, rc.Repo, r)
	if err != nil {
		return nil, fmt.Errorf("Failed to create release: %s", err)
	}

	fmt.Printf("Successfully created %s release\n", rc.Tag)
	return release, nil
}

func (rc *releaseClient) editRelease(id int64) (*gitea.Release, error) {
	r := gitea.EditReleaseOption{
		TagName:      rc.Tag,
		IsDraft:      &rc.Draft,
		IsPrerelease: &rc.Prerelease,
		Title:        rc.Title,
		Note:         rc.Note,
	}

	release, _, err := rc.Client.EditRelease(rc.Owner, rc.Repo, id, r)
	if err != nil {
		return nil, fmt.Errorf("Failed to edit release: %s", err)
	}

	fmt.Printf("Successfully edited %s release\n", rc.Tag)
	return release, nil
}

func (rc *releaseClient) uploadFiles(releaseID int64, files []string) error {
	attachments, _, err := rc.Client.ListReleaseAttachments(rc.Owner, rc.Repo, releaseID, gitea.ListReleaseAttachmentsOptions{})

	if err != nil {
		return fmt.Errorf("Failed to fetch existing assets: %s", err)
	}

	var uploadFiles []string

files:
	for _, file := range files {
		for _, attachment := range attachments {
			if attachment.Name == path.Base(file) {
				switch rc.FileExists {
				case "overwrite":
					// do nothing
				case "fail":
					return fmt.Errorf("Asset file %s already exists", path.Base(file))
				case "skip":
					fmt.Printf("Skipping pre-existing %s artifact\n", attachment.Name)
					continue files
				default:
					return fmt.Errorf("Internal error, unknown file_exist value %s", rc.FileExists)
				}
			}
		}

		uploadFiles = append(uploadFiles, file)
	}

	for _, file := range uploadFiles {
		handle, err := os.Open(file)

		if err != nil {
			return fmt.Errorf("Failed to read %s artifact: %s", file, err)
		}

		for _, attachment := range attachments {
			if _, err := rc.Client.DeleteReleaseAttachment(rc.Owner, rc.Repo, releaseID, attachment.ID); err != nil {
				return fmt.Errorf("Failed to delete %s artifact: %s", file, err)
			}

			fmt.Printf("Successfully deleted old %s artifact\n", attachment.Name)
		}

		if _, _, err = rc.Client.CreateReleaseAttachment(rc.Owner, rc.Repo, releaseID, handle, path.Base(file)); err != nil {
			return fmt.Errorf("Failed to upload %s artifact: %s", file, err)
		}

		fmt.Printf("Successfully uploaded %s artifact\n", file)
	}

	return nil
}
