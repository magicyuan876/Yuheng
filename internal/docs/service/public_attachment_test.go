package service

import (
	"fmt"
	"net/url"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/magicyuan876/yuheng/internal/docs/model"
)

// newPublishingEnv is a space with storage for attachments and public sharing
// switched on.
func newPublishingEnv(t *testing.T) *attachEnv {
	t.Helper()
	files := newMemFiles()
	tenants := &fakeTenants{}
	p := newPageEnvWith(t, func(d *Deps) {
		d.Storage = fakeStorage{files: files}
		d.Tenants = tenants
		d.PublicSharing = true
	})
	return &attachEnv{pageEnv: p, files: files, tenants: tenants}
}

// pageWithImage creates a page whose body shows one uploaded image.
func (e *attachEnv) pageWithImage(t *testing.T, parent *string, title string) (*PageView, *AttachmentView) {
	t.Helper()
	page := e.create(t, e.alice, parent, title)
	img, err := e.put(t, e.alice, title+".png", testPNG(t, 8, 8, 40), "")
	require.NoError(t, err)
	e.save(t, page.ID, 0, []byte(fmt.Sprintf(
		`{"type":"doc","content":[{"type":"image","attrs":{"attachmentId":%q,"src":null}}]}`, img.ID)))
	return page, img
}

var imgSrc = regexp.MustCompile(`<img[^>]* src="([^"]+)"`)

// srcIn returns the image address a rendered page points its visitor at.
func srcIn(t *testing.T, html string) *url.URL {
	t.Helper()
	m := imgSrc.FindStringSubmatch(html)
	require.NotNil(t, m, "the page renders its image: %s", html)
	u, err := url.Parse(m[1])
	require.NoError(t, err)
	return u
}

func (e *attachEnv) fetchShared(key, aid, sig string) (*ServeResult, error) {
	reach, err := e.svc.Pages.ShareAttachmentReach(ctx(), key, aid, sig)
	if err != nil {
		return nil, err
	}
	return e.svc.Files.FetchPublished(ctx(), reach, aid, 0)
}

func TestASharedPagesImagesReachItsVisitor(t *testing.T) {
	e := newPublishingEnv(t)
	page, img := e.pageWithImage(t, nil, "Notes")
	link := e.publish(t, page.ID, CreateShareInput{})

	src := srcIn(t, e.visit(t, link.Key, "", "").Page.HTML)
	assert.Equal(t, "/api/v1/docs/public/"+link.Key+"/attachments/"+img.ID, src.Path,
		"not the member route, which a visitor's browser cannot use")
	assert.Empty(t, src.RawQuery, "a link without a password needs no signature")

	res, err := e.fetchShared(link.Key, img.ID, "")
	require.NoError(t, err)
	assert.Equal(t, testPNG(t, 8, 8, 40), readAll(t, res))

	require.NoError(t, e.svc.Pages.RevokeShare(ctx(), e.alice, e.decision(t, e.alice, page.ID), link.ID))
	_, err = e.fetchShared(link.Key, img.ID, "")
	assert.Equal(t, 404, httpCode(t, err), "revoking the link stops its files at once")
}

func TestALinkServesOnlyTheFilesOfPagesItPublishes(t *testing.T) {
	e := newPublishingEnv(t)
	parent, _ := e.pageWithImage(t, nil, "Parent")
	_, childImg := e.pageWithImage(t, &parent.ID, "Child")
	secret, secretImg := e.pageWithImage(t, &parent.ID, "Secret")
	_, elsewhereImg := e.pageWithImage(t, nil, "Elsewhere")
	e.cut(t, e.alice, secret.ID)

	alone := e.publish(t, parent.ID, CreateShareInput{})
	_, err := e.fetchShared(alone.Key, childImg.ID, "")
	assert.Equal(t, 404, httpCode(t, err), "children are not in a link that leaves them out")

	withKids := e.publish(t, parent.ID, CreateShareInput{IncludeChildren: true})
	res, err := e.fetchShared(withKids.Key, childImg.ID, "")
	require.NoError(t, err)
	_ = res.Close()

	_, err = e.fetchShared(withKids.Key, secretImg.ID, "")
	assert.Equal(t, 404, httpCode(t, err), "a restricted child's files stay private")
	_, err = e.fetchShared(withKids.Key, elsewhereImg.ID, "")
	assert.Equal(t, 404, httpCode(t, err), "naming another page's file does not reach it")

	unbound, err := e.put(t, e.alice, "loose.png", testPNG(t, 8, 8, 41), "")
	require.NoError(t, err)
	_, err = e.fetchShared(withKids.Key, unbound.ID, "")
	assert.Equal(t, 404, httpCode(t, err), "an upload not on any page is on no published page")
}

func TestAProtectedLinksFilesNeedTheSignatureItsPageCarries(t *testing.T) {
	e := newPublishingEnv(t)
	page, img := e.pageWithImage(t, nil, "Notes")
	_, otherImg := e.pageWithImage(t, &page.ID, "Other")
	link := e.publish(t, page.ID, CreateShareInput{Password: "open sesame", IncludeChildren: true})

	unlocked, err := e.svc.Pages.UnlockShare(ctx(), link.Key, "open sesame")
	require.NoError(t, err)
	src := srcIn(t, e.visit(t, link.Key, unlocked.UnlockToken, "").Page.HTML)
	sig := src.Query().Get("sig")
	require.NotEmpty(t, sig, "a protected link's page signs its files")
	assert.NotEqual(t, unlocked.UnlockToken, sig, "the unlock token itself never goes into a URL")

	_, err = e.fetchShared(link.Key, img.ID, "")
	assert.Equal(t, 404, httpCode(t, err), "without the signature the file is not served")
	_, err = e.fetchShared(link.Key, img.ID, unlocked.UnlockToken)
	assert.Equal(t, 404, httpCode(t, err), "an unlock token is not an attachment signature")
	_, err = e.fetchShared(link.Key, otherImg.ID, sig)
	assert.Equal(t, 404, httpCode(t, err), "a signature opens one file, not its neighbours")

	res, err := e.fetchShared(link.Key, img.ID, sig)
	require.NoError(t, err)
	_ = res.Close()

	next := "second one"
	_, err = e.svc.Pages.UpdateShare(ctx(), e.alice, e.decision(t, e.alice, page.ID),
		link.ID, UpdateShareInput{Password: &next})
	require.NoError(t, err)
	_, err = e.fetchShared(link.Key, img.ID, sig)
	assert.Equal(t, 404, httpCode(t, err), "changing the password voids every signature")
}

func TestAPublicSpacesImagesReachItsVisitor(t *testing.T) {
	e := newPublishingEnv(t)
	page, img := e.pageWithImage(t, nil, "Home")
	secret, secretImg := e.pageWithImage(t, nil, "Secret")
	e.cut(t, e.alice, secret.ID)
	space := e.makePublic(t)

	got, err := e.svc.Pages.PublicSpacePage(ctx(), space.ID, page.ShortID)
	require.NoError(t, err)
	assert.Equal(t, "/api/v1/docs/public-spaces/"+space.ID+"/attachments/"+img.ID, srcIn(t, got.HTML).Path)

	reach, err := e.svc.Pages.PublicSpaceAttachmentReach(ctx(), space.ID)
	require.NoError(t, err)
	res, err := e.svc.Files.FetchPublished(ctx(), reach, img.ID, 0)
	require.NoError(t, err)
	assert.Equal(t, testPNG(t, 8, 8, 40), readAll(t, res))

	_, err = e.svc.Files.FetchPublished(ctx(), reach, secretImg.ID, 0)
	assert.Equal(t, 404, httpCode(t, err), "a restricted page of a public space is not public")

	vis := model.VisibilityPrivate
	_, err = e.svc.Spaces.Update(ctx(), e.alice, space, UpdateSpaceInput{Visibility: &vis})
	require.NoError(t, err)
	_, err = e.svc.Pages.PublicSpaceAttachmentReach(ctx(), space.ID)
	assert.Equal(t, 404, httpCode(t, err), "making the space private stops its files")
}
