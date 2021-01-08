package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/mrparkers/terraform-provider-keycloak/keycloak"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestAccKeycloakUser_basic(t *testing.T) {
	t.Parallel()
	realmName := "terraform-" + acctest.RandString(10)
	username := "terraform-user-" + acctest.RandString(10)
	attributeName := "terraform-attribute-" + acctest.RandString(10)
	attributeValue := acctest.RandString(250)

	resourceName := "keycloak_user.user"

	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckKeycloakUserDestroy(),
		Steps: []resource.TestStep{
			{
				Config: testKeycloakUser_basic(realmName, username, attributeName, attributeValue),
				Check:  testAccCheckKeycloakUserExists(resourceName),
			},
			{
				ResourceName:        resourceName,
				ImportState:         true,
				ImportStateVerify:   true,
				ImportStateIdPrefix: realmName + "/",
			},
		},
	})
}

func TestAccKeycloakUser_withInitialPassword(t *testing.T) {
	t.Parallel()
	realmName := "terraform-" + acctest.RandString(10)
	username := "terraform-user-" + acctest.RandString(10)
	password := "terraform-password-" + acctest.RandString(10)
	clientId := "terraform-client-" + acctest.RandString(10)

	resourceName := "keycloak_user.user"

	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckKeycloakUserDestroy(),
		Steps: []resource.TestStep{
			{
				Config: testKeycloakUser_initialPassword(realmName, username, password, clientId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckKeycloakUserExists(resourceName),
					testAccCheckKeycloakUserInitialPasswordLogin(realmName, username, password, clientId),
				),
			},
		},
	})
}

func TestAccKeycloakUser_createAfterManualDestroy(t *testing.T) {
	t.Parallel()
	var user = &keycloak.User{}

	realmName := "terraform-" + acctest.RandString(10)
	username := "terraform-user-" + acctest.RandString(10)
	attributeName := "terraform-attribute-" + acctest.RandString(10)
	attributeValue := acctest.RandString(250)
	resourceName := "keycloak_user.user"

	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckKeycloakUserDestroy(),
		Steps: []resource.TestStep{
			{
				Config: testKeycloakUser_basic(realmName, username, attributeName, attributeValue),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckKeycloakUserExists(resourceName),
					testAccCheckKeycloakUserFetch(resourceName, user),
				),
			},
			{
				PreConfig: func() {
					keycloakClient := testAccProvider.Meta().(*keycloak.KeycloakClient)

					err := keycloakClient.DeleteUser(user.RealmId, user.Id)
					if err != nil {
						t.Fatal(err)
					}
				},
				Config: testKeycloakUser_basic(realmName, username, attributeName, attributeValue),
				Check:  testAccCheckKeycloakUserExists(resourceName),
			},
		},
	})
}

func TestAccKeycloakUser_updateRealm(t *testing.T) {
	t.Parallel()
	realmOne := "terraform-" + acctest.RandString(10)
	realmTwo := "terraform-" + acctest.RandString(10)
	username := "terraform-user-" + acctest.RandString(10)

	resourceName := "keycloak_user.user"

	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckKeycloakUserDestroy(),
		Steps: []resource.TestStep{
			{
				Config: testKeycloakUser_updateRealmBefore(realmOne, realmTwo, username),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckKeycloakUserExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "realm_id", realmOne),
				),
			},
			{
				Config: testKeycloakUser_updateRealmAfter(realmOne, realmTwo, username),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckKeycloakUserExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "realm_id", realmTwo),
				),
			},
		},
	})
}

func TestAccKeycloakUser_updateUsername(t *testing.T) {
	t.Parallel()
	realmName := "terraform-" + acctest.RandString(10)
	usernameOne := "terraform-user-" + acctest.RandString(10)
	usernameTwo := "terraform-user-" + acctest.RandString(10)
	attributeName := "terraform-attribute-" + acctest.RandString(10)
	attributeValue := acctest.RandString(250)

	resourceName := "keycloak_user.user"

	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckKeycloakUserDestroy(),
		Steps: []resource.TestStep{
			{
				Config: testKeycloakUser_basic(realmName, usernameOne, attributeName, attributeValue),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckKeycloakUserExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "username", usernameOne),
				),
			},
			{
				Config: testKeycloakUser_basic(realmName, usernameTwo, attributeName, attributeValue),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckKeycloakUserExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "username", usernameTwo),
				),
			},
		},
	})
}

func TestAccKeycloakUser_updateWithInitialPasswordChangeDoesNotReset(t *testing.T) {
	t.Parallel()
	realmName := "terraform-" + acctest.RandString(10)
	username := "terraform-user-" + acctest.RandString(10)
	passwordOne := "terraform-password1-" + acctest.RandString(10)
	passwordTwo := "terraform-password2-" + acctest.RandString(10)
	clientId := "terraform-client-" + acctest.RandString(10)

	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckKeycloakUserDestroy(),
		Steps: []resource.TestStep{
			{
				Config: testKeycloakUser_initialPassword(realmName, username, passwordOne, clientId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckKeycloakUserInitialPasswordLogin(realmName, username, passwordOne, clientId),
				),
			},
			{
				Config: testKeycloakUser_initialPassword(realmName, username, passwordTwo, clientId),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckKeycloakUserInitialPasswordLogin(realmName, username, passwordOne, clientId),
				),
			},
		},
	})
}

func TestAccKeycloakUser_updateInPlace(t *testing.T) {
	t.Parallel()
	userOne := &keycloak.User{
		RealmId:       "terraform-" + acctest.RandString(10),
		Username:      "terraform-user-" + acctest.RandString(10),
		Email:         fmt.Sprintf("%s@gmail.com", acctest.RandString(10)),
		FirstName:     acctest.RandString(10),
		LastName:      acctest.RandString(10),
		Enabled:       randomBool(),
		EmailVerified: randomBool(),
	}

	userTwo := &keycloak.User{
		RealmId:       userOne.RealmId,
		Username:      userOne.Username,
		Email:         fmt.Sprintf("%s@gmail.com", acctest.RandString(10)),
		FirstName:     acctest.RandString(10),
		LastName:      acctest.RandString(10),
		Enabled:       randomBool(),
		EmailVerified: !userOne.EmailVerified,
	}

	resourceName := "keycloak_user.user"

	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckKeycloakUserDestroy(),
		Steps: []resource.TestStep{
			{
				Config: testKeycloakUser_fromInterface(userOne),
				Check:  testAccCheckKeycloakUserExists(resourceName),
			},
			{
				Config: testKeycloakUser_fromInterface(userTwo),
				Check:  testAccCheckKeycloakUserExists(resourceName),
			},
		},
	})
}

func TestAccKeycloakUser_unsetOptionalAttributes(t *testing.T) {
	t.Parallel()
	attributeName := "terraform-attribute-" + acctest.RandString(10)
	userWithOptionalAttributes := &keycloak.User{
		RealmId:   "terraform-" + acctest.RandString(10),
		Username:  "terraform-user-" + acctest.RandString(10),
		Email:     fmt.Sprintf("%s@gmail.com", acctest.RandString(10)),
		FirstName: acctest.RandString(10),
		LastName:  acctest.RandString(10),
		Enabled:   randomBool(),
		Attributes: map[string][]string{
			attributeName: {
				acctest.RandString(230),
				acctest.RandString(12),
			},
		},
	}

	resourceName := "keycloak_user.user"

	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckKeycloakUserDestroy(),
		Steps: []resource.TestStep{
			{
				Config: testKeycloakUser_fromInterface(userWithOptionalAttributes),
				Check:  testAccCheckKeycloakUserExists(resourceName),
			},
			{
				Config: testKeycloakUser_basic(userWithOptionalAttributes.RealmId, userWithOptionalAttributes.Username, attributeName, strings.Join(userWithOptionalAttributes.Attributes[attributeName], "")),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckKeycloakUserExists(resourceName),
					resource.TestCheckResourceAttr(resourceName, "email", ""),
					resource.TestCheckResourceAttr(resourceName, "first_name", ""),
					resource.TestCheckResourceAttr(resourceName, "last_name", ""),
				),
			},
		},
	})
}

func TestAccKeycloakUser_validateLowercaseUsernames(t *testing.T) {
	t.Parallel()
	realmName := "terraform-" + acctest.RandString(10)
	username := "terraform-user-" + strings.ToUpper(acctest.RandString(10))
	attributeName := "terraform-attribute-" + acctest.RandString(10)
	attributeValue := acctest.RandString(250)

	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckKeycloakUserDestroy(),
		Steps: []resource.TestStep{
			{
				Config:      testKeycloakUser_basic(realmName, username, attributeName, attributeValue),
				ExpectError: regexp.MustCompile("expected username .+ to be all lowercase"),
			},
		},
	})
}

func TestAccKeycloakUser_federatedLink(t *testing.T) {
	t.Parallel()
	sourceUserName := "terraform-source-user-" + acctest.RandString(10)
	sourceUserName2 := "terraform-source-user2-" + acctest.RandString(10)
	destinationRealmName := "terraform-dest-" + acctest.RandString(10)

	resourceName := "keycloak_user.destination_user"

	resource.Test(t, resource.TestCase{
		ProviderFactories: testAccProviderFactories,
		PreCheck:          func() { testAccPreCheck(t) },
		CheckDestroy:      testAccCheckKeycloakUserDestroy(),
		Steps: []resource.TestStep{
			{
				Config: testKeycloakUser_FederationLink(sourceUserName, destinationRealmName),
				Check:  testAccCheckKeycloakUserHasFederationLinkWithSourceUserName(resourceName, sourceUserName),
			},
			{
				Config: testKeycloakUser_FederationLink(sourceUserName2, destinationRealmName),
				Check:  testAccCheckKeycloakUserHasFederationLinkWithSourceUserName(resourceName, sourceUserName2),
			},
		},
	})
}

func testAccCheckKeycloakUserHasFederationLinkWithSourceUserName(resourceName, sourceUserName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		fetchedUser, err := getUserFromState(s, resourceName)
		if err != nil {
			return err
		}

		var found bool = false
		for _, federatedIdentity := range fetchedUser.FederatedIdentities {
			if federatedIdentity.UserName == sourceUserName {
				found = true
			}
			if !found {
				return fmt.Errorf("user had unexpected federatedLink %s or unexpected username %s", federatedIdentity.IdentityProvider, federatedIdentity.UserName)
			}
		}

		if !found {
			return fmt.Errorf("user had no federatedLink, but one was expected")
		}

		return nil
	}
}

func testAccCheckKeycloakUserExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		_, err := getUserFromState(s, resourceName)
		if err != nil {
			return err
		}

		return nil
	}
}

func testAccCheckKeycloakUserFetch(resourceName string, user *keycloak.User) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		fetchedUser, err := getUserFromState(s, resourceName)
		if err != nil {
			return err
		}

		user.Id = fetchedUser.Id
		user.RealmId = fetchedUser.RealmId

		return nil
	}
}

func testAccCheckKeycloakUserInitialPasswordLogin(realmName string, username string, password string, clientId string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		httpClient := &http.Client{}

		resourceUrl := fmt.Sprintf("%s/auth/realms/%s/protocol/openid-connect/token", os.Getenv("KEYCLOAK_URL"), realmName)

		form := url.Values{}
		form.Add("username", username)
		form.Add("password", password)
		form.Add("client_id", clientId)
		form.Add("grant_type", "password")

		request, err := http.NewRequest(http.MethodPost, resourceUrl, strings.NewReader(form.Encode()))
		if err != nil {
			return err
		}
		request.Header.Add("Content-Type", "application/x-www-form-urlencoded")

		response, err := httpClient.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()

		if response.StatusCode != http.StatusOK {
			body, _ := ioutil.ReadAll(response.Body)
			return fmt.Errorf("user with username %s cannot login with password %s\n body: %s", username, password, string(body))
		}

		return nil
	}
}

func testAccCheckKeycloakUserDestroy() resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "keycloak_user" {
				continue
			}

			id := rs.Primary.ID
			realm := rs.Primary.Attributes["realm_id"]

			keycloakClient := testAccProvider.Meta().(*keycloak.KeycloakClient)

			user, _ := keycloakClient.GetUser(realm, id)
			if user != nil {
				return fmt.Errorf("user with id %s still exists", id)
			}
		}

		return nil
	}
}

func getUserFromState(s *terraform.State, resourceName string) (*keycloak.User, error) {
	keycloakClient := testAccProvider.Meta().(*keycloak.KeycloakClient)

	rs, ok := s.RootModule().Resources[resourceName]
	if !ok {
		return nil, fmt.Errorf("resource not found: %s", resourceName)
	}

	id := rs.Primary.ID
	realm := rs.Primary.Attributes["realm_id"]

	user, err := keycloakClient.GetUser(realm, id)
	if err != nil {
		return nil, fmt.Errorf("error getting user with id %s: %s", id, err)
	}

	return user, nil
}

func testKeycloakUser_basic(realm, username, attributeName, attributeValue string) string {
	return fmt.Sprintf(`
resource "keycloak_realm" "realm" {
	realm = "%s"
}

resource "keycloak_user" "user" {
	realm_id = "${keycloak_realm.realm.id}"
	username = "%s"
	attributes = {
		"%s" = "%s"
	}
}
	`, realm, username, attributeName, attributeValue)
}

func testKeycloakUser_initialPassword(realm, username string, password string, clientId string) string {
	return fmt.Sprintf(`
resource "keycloak_realm" "realm" {
	realm = "%s"
}

resource "keycloak_openid_client" "client" {
	realm_id                     = "${keycloak_realm.realm.id}"
	client_id                    = "%s"

	name                         = "test client"
	enabled                      = true

	access_type                  = "PUBLIC"
	direct_access_grants_enabled = true
}

resource "keycloak_user" "user" {
	realm_id         = "${keycloak_realm.realm.id}"
	username         = "%s"
	initial_password {
		value = "%s"
		temporary = false
	}
}
	`, realm, clientId, username, password)
}

func testKeycloakUser_updateRealmBefore(realmOne, realmTwo, username string) string {
	return fmt.Sprintf(`
resource "keycloak_realm" "realm_1" {
	realm = "%s"
}

resource "keycloak_realm" "realm_2" {
	realm = "%s"
}

resource "keycloak_user" "user" {
	realm_id  = "${keycloak_realm.realm_1.id}"
	username  = "%s"
}
	`, realmOne, realmTwo, username)
}

func testKeycloakUser_updateRealmAfter(realmOne, realmTwo, username string) string {
	return fmt.Sprintf(`
resource "keycloak_realm" "realm_1" {
	realm = "%s"
}

resource "keycloak_realm" "realm_2" {
	realm = "%s"
}

resource "keycloak_user" "user" {
	realm_id  = "${keycloak_realm.realm_2.id}"
	username  = "%s"
}
	`, realmOne, realmTwo, username)
}

func testKeycloakUser_fromInterface(user *keycloak.User) string {
	return fmt.Sprintf(`
resource "keycloak_realm" "realm" {
	realm = "%s"
}

resource "keycloak_user" "user" {
	realm_id       = "${keycloak_realm.realm.id}"
	username       = "%s"

	email          = "%s"
	first_name     = "%s"
	last_name      = "%s"
	enabled        = %t
	email_verified = "%t"
}
	`, user.RealmId, user.Username, user.Email, user.FirstName, user.LastName, user.Enabled, user.EmailVerified)
}

func testKeycloakUser_FederationLink(sourceRealmUserName, destinationRealmId string) string {
	return fmt.Sprintf(`
resource "keycloak_realm" "source_realm" {
  realm   = "source_test_realm"
  enabled = true
}

resource "keycloak_openid_client" "destination_client" {
  realm_id                 = "${keycloak_realm.source_realm.id}"
  client_id                = "destination_client"
  client_secret            = "secret"
  access_type              = "CONFIDENTIAL"
  standard_flow_enabled    = true
  valid_redirect_uris = [
    "http://localhost:8080/*",
  ]
}

resource "keycloak_user" "source_user" {
  realm_id   = "${keycloak_realm.source_realm.id}"
  username   = "%s"
  initial_password {
    value     = "source"
    temporary = false
  }
}

resource "keycloak_realm" "destination_realm" {
  realm   = "%s"
  enabled = true
}

resource keycloak_oidc_identity_provider source_oidc_idp {
  realm              = "${keycloak_realm.destination_realm.id}"
  alias              = "source"
  authorization_url  = "http://localhost:8080/auth/realms/${keycloak_realm.source_realm.id}/protocol/openid-connect/auth"
  token_url          = "http://localhost:8080/auth/realms/${keycloak_realm.source_realm.id}/protocol/openid-connect/token"
  client_id          = "${keycloak_openid_client.destination_client.client_id}"
  client_secret      = "${keycloak_openid_client.destination_client.client_secret}"
  default_scopes     = "openid"
}

resource "keycloak_user" "destination_user" {
  realm_id   = "${keycloak_realm.destination_realm.id}"
  username   = "my_destination_username"
  federated_identity {
    identity_provider = "${keycloak_oidc_identity_provider.source_oidc_idp.alias}"
    user_id           = "${keycloak_user.source_user.id}"
    user_name         = "${keycloak_user.source_user.username}"
  }
}
	`, sourceRealmUserName, destinationRealmId)
}
