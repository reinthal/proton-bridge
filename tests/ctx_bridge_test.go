package tests

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http/cookiejar"

	"github.com/ProtonMail/proton-bridge/v2/internal/bridge"
	"github.com/ProtonMail/proton-bridge/v2/internal/cookies"
	"github.com/ProtonMail/proton-bridge/v2/internal/events"
	"github.com/ProtonMail/proton-bridge/v2/internal/useragent"
	"github.com/ProtonMail/proton-bridge/v2/internal/vault"
	"gitlab.protontech.ch/go/liteapi"
)

func (t *testCtx) startBridge() error {
	// Bridge will enable the proxy by default at startup.
	t.mocks.ProxyCtl.EXPECT().AllowProxy()

	// Get the path to the vault.
	vaultDir, err := t.locator.ProvideSettingsPath()
	if err != nil {
		return err
	}

	// Get the default gluon path.
	gluonDir, err := t.locator.ProvideGluonPath()
	if err != nil {
		return err
	}

	// Create the vault.
	vault, corrupt, err := vault.New(vaultDir, gluonDir, t.storeKey)
	if err != nil {
		return err
	} else if corrupt {
		return fmt.Errorf("vault is corrupt")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}

	persister, err := cookies.NewCookieJar(jar, vault)
	if err != nil {
		return err
	}

	// Create the bridge.
	bridge, err := bridge.New(
		t.locator,
		vault,
		t.mocks.Autostarter,
		t.mocks.Updater,
		t.version,

		t.api.GetHostURL(),
		persister,
		useragent.New(),
		t.mocks.TLSReporter,
		liteapi.NewDialer(t.netCtl, &tls.Config{InsecureSkipVerify: true}).GetRoundTripper(),
		t.mocks.ProxyCtl,

		false,
		false,
		false,
	)
	if err != nil {
		return err
	}

	// Save the bridge t.
	t.bridge = bridge

	// Connect the event channels.
	t.loginCh = chToType[events.Event, events.UserLoggedIn](bridge.GetEvents(events.UserLoggedIn{}))
	t.logoutCh = chToType[events.Event, events.UserLoggedOut](bridge.GetEvents(events.UserLoggedOut{}))
	t.deletedCh = chToType[events.Event, events.UserDeleted](bridge.GetEvents(events.UserDeleted{}))
	t.deauthCh = chToType[events.Event, events.UserDeauth](bridge.GetEvents(events.UserDeauth{}))
	t.addrCreatedCh = chToType[events.Event, events.UserAddressCreated](bridge.GetEvents(events.UserAddressCreated{}))
	t.addrDeletedCh = chToType[events.Event, events.UserAddressDeleted](bridge.GetEvents(events.UserAddressDeleted{}))
	t.syncStartedCh = chToType[events.Event, events.SyncStarted](bridge.GetEvents(events.SyncStarted{}))
	t.syncFinishedCh = chToType[events.Event, events.SyncFinished](bridge.GetEvents(events.SyncFinished{}))
	t.forcedUpdateCh = chToType[events.Event, events.UpdateForced](bridge.GetEvents(events.UpdateForced{}))
	t.connStatusCh, _ = bridge.GetEvents(events.ConnStatusUp{}, events.ConnStatusDown{})
	t.updateCh, _ = bridge.GetEvents(events.UpdateAvailable{}, events.UpdateNotAvailable{}, events.UpdateInstalled{}, events.UpdateForced{})

	return nil
}

func (t *testCtx) stopBridge() error {
	if err := t.bridge.Close(context.Background()); err != nil {
		return err
	}

	t.bridge = nil

	return nil
}
