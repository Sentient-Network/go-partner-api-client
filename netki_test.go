package netki

import (
	"fmt"
	"github.com/bitly/go-simplejson"
	"github.com/bmizerany/assert"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// Utility Functions
func stringInArray(text string, list []string) bool {
	for _, v := range list {
		if v == text {
			return true
		}
	}
	return false
}

// Setup Mocks
type MockNetkiRequester struct {
	returnData  *simplejson.Json
	returnError error

	calledUri, calledMethod, calledBodyData string
}

func (n *MockNetkiRequester) ProcessRequest(partner *NetkiPartner, uri string, method string, bodyData string) (*simplejson.Json, error) {
	n.calledUri = uri
	n.calledMethod = method
	n.calledBodyData = bodyData

	return n.returnData, n.returnError
}

// Setup WalletName Base
func getWalletName() WalletName {
	wn := WalletName{}
	wn.DomainName = "domain.com"
	wn.ExternalId = "ext_id"
	wn.Name = "wallet"

	wn.Wallets = make([]Wallet, 0)
	wallet := Wallet{"btc", "1btcaddress"}
	wn.Wallets = append(wn.Wallets, wallet)
	return wn
}

// Setup mockRequester
func getMockRequester(returnData string, returnError error) *MockNetkiRequester {
	newRequester := new(MockNetkiRequester)
	if returnData != "" {
		localJson, err := simplejson.NewJson([]byte(returnData))
		if err != nil {
			fmt.Println("JSON FORMAT ERROR: ", err)
		}
		newRequester.returnData = localJson
	}
	newRequester.returnError = returnError
	return newRequester
}

// Setup Mock HTTP Client/Server
func setupHttp(code int, contentType string, body string) (*httptest.Server, *http.Client) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(code)
		fmt.Fprintln(w, body)
	}))

	proxyUrl, _ := url.Parse(server.URL)
	transport := &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
	httpClient := &http.Client{Transport: transport}
	return server, httpClient
}

func TestUrlEncode(t *testing.T) {
	assert.Equal(t, "Test%20Partner%201", urlEncode("Test Partner 1"))
	assert.Equal(t, "TestPartner", urlEncode("TestPartner"))
}

// ProcessRequest()
func TestProcessRequest(t *testing.T) {
	server, client := setupHttp(200, "application/json", `{"success":true,"message":"my message"}`)
	defer server.Close()

	requester := &NetkiRequester{HTTPClient: client}
	result, err := requester.ProcessRequest(&NetkiPartner{}, "http://domain.com/uri", "GET", "")

	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, result)
	assert.Equal(t, true, result.Get("success").MustBool())
	assert.Equal(t, "my message", result.Get("message").MustString())
}

func TestProcessRequestDelete204(t *testing.T) {
	server, client := setupHttp(204, "application/json", `{"success":true,"message":"my message"}`)
	defer server.Close()

	requester := &NetkiRequester{HTTPClient: client}
	result, err := requester.ProcessRequest(&NetkiPartner{}, "http://domain.com/uri", "DELETE", "")

	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, result)
	assert.Equal(t, &simplejson.Json{}, result)
}

func TestProcessRequestBadContentType(t *testing.T) {
	server, client := setupHttp(200, "text/plain", "")
	defer server.Close()

	requester := &NetkiRequester{HTTPClient: client}
	result, err := requester.ProcessRequest(&NetkiPartner{}, "http://domain.com/uri", "GET", "")

	assert.NotEqual(t, nil, err)
	assert.NotEqual(t, nil, result)
	assert.Equal(t, &simplejson.Json{}, result)
	assert.Equal(t, "HTTP Response Contains Invalid Content-Type: text/plain", err.Error())
}

func TestProcessRequestNotJSONData(t *testing.T) {
	server, client := setupHttp(200, "application/json", "")
	defer server.Close()

	requester := &NetkiRequester{HTTPClient: client}
	result, err := requester.ProcessRequest(&NetkiPartner{}, "http://domain.com/uri", "GET", "")

	assert.NotEqual(t, nil, err)
	assert.NotEqual(t, nil, result)
	assert.Equal(t, &simplejson.Json{}, result)
	assert.Equal(t, "Error Retrieving JSON Data: EOF", err.Error())
}

func TestProcessRequestSuccessFalseNoFailures(t *testing.T) {
	server, client := setupHttp(200, "application/json", `{"success":"false","message":"Error Message"}`)
	defer server.Close()

	requester := &NetkiRequester{HTTPClient: client}
	result, err := requester.ProcessRequest(&NetkiPartner{}, "http://domain.com/uri", "GET", "")

	assert.NotEqual(t, nil, err)
	assert.NotEqual(t, nil, result)
	assert.Equal(t, &simplejson.Json{}, result)
	assert.Equal(t, "Error Message", err.Error())
}

func TestProcessRequestSuccessFalseEmptyFailures(t *testing.T) {
	server, client := setupHttp(200, "application/json", `{"success":"false","message":"Error Message","failures":[]}`)
	defer server.Close()

	requester := &NetkiRequester{HTTPClient: client}
	result, err := requester.ProcessRequest(&NetkiPartner{}, "http://domain.com/uri", "GET", "")

	assert.NotEqual(t, nil, err)
	assert.NotEqual(t, nil, result)
	assert.Equal(t, &simplejson.Json{}, result)
	assert.Equal(t, "Error Message [FAILURES: ]", err.Error())
}

func TestProcessRequestSuccessFalseWithFailures(t *testing.T) {
	server, client := setupHttp(200, "application/json", `{"success":"false","message":"Error Message","failures":[{"message":"fail1"},{"message":"fail2"}]}`)
	defer server.Close()

	requester := &NetkiRequester{HTTPClient: client}
	result, err := requester.ProcessRequest(&NetkiPartner{}, "http://domain.com/uri", "GET", "")

	assert.NotEqual(t, nil, err)
	assert.NotEqual(t, nil, result)
	assert.Equal(t, &simplejson.Json{}, result)
	assert.Equal(t, "Error Message [FAILURES: fail1, fail2]", err.Error())
}

// WalletName Tests
func TestGetAddress(t *testing.T) {
	wn := getWalletName()

	assert.Equal(t, "1btcaddress", wn.GetAddress("btc"))
	assert.Equal(t, "", wn.GetAddress("no_currency"))
}

func TestUsedCurrencies(t *testing.T) {
	wn := getWalletName()
	wallet2 := Wallet{"dgc", "Daddr"}
	wn.Wallets = append(wn.Wallets, wallet2)

	currencies := wn.UsedCurrencies()
	assert.Equal(t, true, stringInArray("btc", currencies))
	assert.Equal(t, true, stringInArray("dgc", currencies))
	assert.Equal(t, false, stringInArray("cur", currencies))
}

func TestSetCurrencyAddress(t *testing.T) {
	wn := getWalletName()

	wn.SetCurrencyAddress("btc", "newaddress")
	assert.Equal(t, "newaddress", wn.GetAddress("btc"))

	wn.SetCurrencyAddress("dgc", "Daddr")
	assert.Equal(t, "Daddr", wn.GetAddress("dgc"))
}

func TestRemoveCurrency(t *testing.T) {
	wn := getWalletName()
	wn.RemoveCurrency("btc")

	assert.Equal(t, "", wn.GetAddress("btc"))
}

// WalletName.Save()
func TestSaveNew(t *testing.T) {
	mockRequester := getMockRequester(`{"wallet_names":[{"id":"my_id"}]}`, nil)
	mockPartner := &NetkiPartner{Requester: mockRequester}

	// Do Our Test
	wn := getWalletName()
	err := wn.Save(mockPartner)

	assert.Equal(t, nil, err)
	assert.Equal(t, "my_id", wn.Id)
	assert.Equal(t, "/v1/partner/walletname", mockRequester.calledUri)
	assert.Equal(t, "POST", mockRequester.calledMethod)
	assert.Equal(t, `{"wallet_names":[{"domain_name":"domain.com","external_id":"ext_id","name":"wallet","wallets":[{"currency":"btc","wallet_address":"1btcaddress"}]}]}`, mockRequester.calledBodyData)
}

func TestSaveExisting(t *testing.T) {
	mockRequester := getMockRequester(`{"wallet_names":[{"id":"my_id"}]}`, nil)
	mockPartner := &NetkiPartner{Requester: mockRequester}

	// Do Our Test
	wn := getWalletName()
	wn.Id = "existingId"
	err := wn.Save(mockPartner)

	assert.Equal(t, nil, err)
	assert.Equal(t, "my_id", wn.Id)
	assert.Equal(t, "/v1/partner/walletname", mockRequester.calledUri)
	assert.Equal(t, "PUT", mockRequester.calledMethod)
	assert.Equal(t, `{"wallet_names":[{"domain_name":"domain.com","external_id":"ext_id","id":"existingId","name":"wallet","wallets":[{"currency":"btc","wallet_address":"1btcaddress"}]}]}`, mockRequester.calledBodyData)
}

func TestSaveErrorResponse(t *testing.T) {
	mockRequester := getMockRequester("", &NetkiError{"Error Message", make([]string, 0)})
	mockPartner := &NetkiPartner{Requester: mockRequester}

	// Do Our Test
	wn := getWalletName()
	err := wn.Save(mockPartner)
	assert.NotEqual(t, nil, err)
	assert.Equal(t, "Error Message", err.Error())
	assert.Equal(t, "/v1/partner/walletname", mockRequester.calledUri)
	assert.Equal(t, "POST", mockRequester.calledMethod)
	assert.Equal(t, `{"wallet_names":[{"domain_name":"domain.com","external_id":"ext_id","name":"wallet","wallets":[{"currency":"btc","wallet_address":"1btcaddress"}]}]}`, mockRequester.calledBodyData)
}

func TestDeleteGoRight(t *testing.T) {
	mockRequester := getMockRequester("", nil)
	mockPartner := &NetkiPartner{Requester: mockRequester}

	wn := getWalletName()
	wn.Id = "existingId"
	err := wn.Delete(mockPartner)

	assert.Equal(t, nil, err)
	assert.Equal(t, "/v1/partner/walletname", mockRequester.calledUri)
	assert.Equal(t, "DELETE", mockRequester.calledMethod)
	assert.Equal(t, `{"wallet_names":[{"domain_name":"domain.com","id":"existingId"}]}`, mockRequester.calledBodyData)
}

func TestDeleteNoId(t *testing.T) {
	mockRequester := getMockRequester("", nil)
	mockPartner := &NetkiPartner{Requester: mockRequester}

	// Do Our Test
	wn := getWalletName()
	err := wn.Delete(mockPartner)
	assert.NotEqual(t, nil, err)
	assert.Equal(t, "WalletName has no ID! Cannot Delete!", err.Error())
	assert.Equal(t, "", mockRequester.calledUri)
}

func TestDeleteError(t *testing.T) {
	mockRequester := getMockRequester("", &NetkiError{"Error Message", make([]string, 0)})
	mockPartner := &NetkiPartner{Requester: mockRequester}

	wn := getWalletName()
	wn.Id = "existingId"
	err := wn.Delete(mockPartner)

	assert.NotEqual(t, nil, err)
	assert.Equal(t, "Error Message", err.Error())
}

// Test NetkiPartner Methods
func TestCreateNewPartner(t *testing.T) {
	mockRequester := getMockRequester(`{"partner":{"id":"partner_id","name":"partner_name"}}`, nil)
	mockPartner := &NetkiPartner{Requester: mockRequester}

	ret, err := mockPartner.CreateNewPartner("Test Partner 1")

	assert.Equal(t, nil, err)
	assert.Equal(t, "/v1/admin/partner/Test%20Partner%201", mockRequester.calledUri)
	assert.Equal(t, "POST", mockRequester.calledMethod)
	assert.Equal(t, "", mockRequester.calledBodyData)

	assert.Equal(t, "partner_id", ret.id)
	assert.Equal(t, "partner_name", ret.partnerName)
}

func TestCreateNewPartnerError(t *testing.T) {
	mockRequester := getMockRequester("", &NetkiError{"Error Message", make([]string, 0)})
	mockPartner := &NetkiPartner{Requester: mockRequester}

	ret, err := mockPartner.CreateNewPartner("Test Partner 1")

	assert.NotEqual(t, nil, err)
	assert.Equal(t, "Error Message", err.Error())
	assert.Equal(t, "/v1/admin/partner/Test%20Partner%201", mockRequester.calledUri)
	assert.Equal(t, "POST", mockRequester.calledMethod)
	assert.Equal(t, "", mockRequester.calledBodyData)

	assert.Equal(t, "", ret.id)
	assert.Equal(t, "", ret.partnerName)
}

func TestGetPartners(t *testing.T) {
	mockRequester := getMockRequester(`{"partners":[{"id":"partner_id","name":"partner_name"}]}`, nil)
	mockPartner := &NetkiPartner{Requester: mockRequester}

	ret, err := mockPartner.GetPartners()

	assert.Equal(t, nil, err)
	assert.Equal(t, "/v1/admin/partner", mockRequester.calledUri)
	assert.Equal(t, "GET", mockRequester.calledMethod)
	assert.Equal(t, "", mockRequester.calledBodyData)

	assert.Equal(t, 1, len(ret))
	assert.Equal(t, "partner_id", ret[0].id)
	assert.Equal(t, "partner_name", ret[0].partnerName)
}

func TestGetPartnersError(t *testing.T) {
	mockRequester := getMockRequester("", &NetkiError{"Error Message", make([]string, 0)})
	mockPartner := &NetkiPartner{Requester: mockRequester}

	ret, err := mockPartner.GetPartners()

	assert.NotEqual(t, nil, err)
	assert.Equal(t, "Error Message", err.Error())
	assert.Equal(t, "/v1/admin/partner", mockRequester.calledUri)
	assert.Equal(t, "GET", mockRequester.calledMethod)
	assert.Equal(t, "", mockRequester.calledBodyData)

	assert.Equal(t, 0, len(ret))
}

func TestDeletePartner(t *testing.T) {
	mockRequester := getMockRequester("", nil)
	mockPartner := &NetkiPartner{Requester: mockRequester}

	err := mockPartner.DeletePartner(Partner{partnerName: "Test Partner 1"})

	assert.Equal(t, nil, err)
	assert.Equal(t, "/v1/admin/partner/Test%20Partner%201", mockRequester.calledUri)
	assert.Equal(t, "DELETE", mockRequester.calledMethod)
	assert.Equal(t, "", mockRequester.calledBodyData)
}

func TestDeletePartnerError(t *testing.T) {
	mockRequester := getMockRequester("", &NetkiError{"Error Message", make([]string, 0)})
	mockPartner := &NetkiPartner{Requester: mockRequester}

	err := mockPartner.DeletePartner(Partner{partnerName: "Test Partner 1"})

	assert.NotEqual(t, nil, err)
	assert.Equal(t, "Error Message", err.Error())
	assert.Equal(t, "/v1/admin/partner/Test%20Partner%201", mockRequester.calledUri)
	assert.Equal(t, "DELETE", mockRequester.calledMethod)
	assert.Equal(t, "", mockRequester.calledBodyData)

}

func TestCreateNewDomain(t *testing.T) {
	mockRequester := getMockRequester(`{"domain_name":"domain.com","nameservers":["ns1.domain.com","ns2.domain.com"],"status":"completed"}`, nil)
	mockPartner := &NetkiPartner{Requester: mockRequester}

	domain, err := mockPartner.CreateNewDomain("domain.com", Partner{id: "partner_id"})

	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, domain)
	assert.Equal(t, "/v1/partner/domain/domain.com", mockRequester.calledUri)
	assert.Equal(t, "POST", mockRequester.calledMethod)
	assert.Equal(t, `{"partner_id":"partner_id"}`, mockRequester.calledBodyData)
	assert.Equal(t, "domain.com", domain.DomainName)
	assert.Equal(t, "completed", domain.Status)
	assert.Equal(t, []string{"ns1.domain.com", "ns2.domain.com"}, domain.Namesevers)
}

func TestCreateNewDomainEmptyPartner(t *testing.T) {
	mockRequester := getMockRequester(`{"domain_name":"domain.com","nameservers":["ns1.domain.com","ns2.domain.com"],"status":"completed"}`, nil)
	mockPartner := &NetkiPartner{Requester: mockRequester}

	domain, err := mockPartner.CreateNewDomain("domain.com", Partner{})

	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, domain)
	assert.Equal(t, "/v1/partner/domain/domain.com", mockRequester.calledUri)
	assert.Equal(t, "POST", mockRequester.calledMethod)
	assert.Equal(t, `{}`, mockRequester.calledBodyData)
	assert.Equal(t, "domain.com", domain.DomainName)
	assert.Equal(t, "completed", domain.Status)
	assert.Equal(t, []string{"ns1.domain.com", "ns2.domain.com"}, domain.Namesevers)
}

func TestCreateNewDomainError(t *testing.T) {
	mockRequester := getMockRequester("", &NetkiError{"Error Message", make([]string, 0)})
	mockPartner := &NetkiPartner{Requester: mockRequester}

	domain, err := mockPartner.CreateNewDomain("domain.com", Partner{})

	assert.NotEqual(t, nil, err)
	assert.NotEqual(t, nil, domain)
	assert.Equal(t, "Error Message", err.Error())
	assert.Equal(t, "/v1/partner/domain/domain.com", mockRequester.calledUri)
	assert.Equal(t, "POST", mockRequester.calledMethod)
	assert.Equal(t, `{}`, mockRequester.calledBodyData)
	assert.Equal(t, "", domain.DomainName)
	assert.Equal(t, "", domain.Status)
	assert.Equal(t, 0, len(domain.Namesevers))
}

func TestGetDomains(t *testing.T) {
	mockRequester := getMockRequester(`{"domains":[{"domain_name":"domain1.com"},{"domain_name":"domain2.com"}]}`, nil)
	mockPartner := &NetkiPartner{Requester: mockRequester}

	domains, err := mockPartner.GetDomains()

	assert.Equal(t, nil, err)
	assert.NotEqual(t, domains, nil)
	assert.Equal(t, "/api/domain", mockRequester.calledUri)
	assert.Equal(t, "GET", mockRequester.calledMethod)
	assert.Equal(t, "", mockRequester.calledBodyData)
	assert.Equal(t, 2, len(domains))
	assert.Equal(t, "domain1.com", domains[0].DomainName)
	assert.Equal(t, "domain2.com", domains[1].DomainName)
}

func TestGetDomainsError(t *testing.T) {
	mockRequester := getMockRequester("", &NetkiError{"Error Message", make([]string, 0)})
	mockPartner := &NetkiPartner{Requester: mockRequester}

	domains, err := mockPartner.GetDomains()

	assert.NotEqual(t, nil, err)
	assert.NotEqual(t, nil, domains)
	assert.Equal(t, "Error Message", err.Error())
	assert.Equal(t, "/api/domain", mockRequester.calledUri)
	assert.Equal(t, "GET", mockRequester.calledMethod)
	assert.Equal(t, "", mockRequester.calledBodyData)
	assert.Equal(t, 0, len(domains))
}

func TestGetDomainStatus(t *testing.T) {
	mockRequester := getMockRequester(`{"status":"completed","delegation_status":true,"delegation_message":"delegation completed","wallet_name_count":42}`, nil)
	mockPartner := &NetkiPartner{Requester: mockRequester}

	domain, err := mockPartner.GetDomainStatus(Domain{DomainName: "domain.com"})

	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, domain)

	assert.Equal(t, "/v1/partner/domain/domain.com", mockRequester.calledUri)
	assert.Equal(t, "GET", mockRequester.calledMethod)
	assert.Equal(t, "", mockRequester.calledBodyData)
	assert.Equal(t, "domain.com", domain.DomainName)
	assert.Equal(t, "completed", domain.Status)
	assert.Equal(t, true, domain.DelegationStatus)
	assert.Equal(t, "delegation completed", domain.DelegationMessage)
	assert.Equal(t, 42, domain.WalletNameCount)
}

func TestGetDomainStatusError(t *testing.T) {
	mockRequester := getMockRequester("", &NetkiError{"Error Message", make([]string, 0)})
	mockPartner := &NetkiPartner{Requester: mockRequester}

	domain, err := mockPartner.GetDomainStatus(Domain{DomainName: "domain.com"})

	assert.NotEqual(t, nil, err)
	assert.NotEqual(t, nil, domain)

	assert.Equal(t, "Error Message", err.Error())
	assert.Equal(t, "/v1/partner/domain/domain.com", mockRequester.calledUri)
	assert.Equal(t, "GET", mockRequester.calledMethod)
	assert.Equal(t, "", mockRequester.calledBodyData)
}

func TestGetDomainDnssec(t *testing.T) {
	mockRequester := getMockRequester(`{"nextroll_date":"2015-06-13T02:35:12.543Z","ds_records":["record 1","record 2"], "public_key_signing_key":"publickey"}`, nil)
	mockPartner := &NetkiPartner{Requester: mockRequester}

	domain, err := mockPartner.GetDomainDnssec(Domain{DomainName: "domain.com"})

	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, domain)
	assert.Equal(t, "/v1/partner/domain/dnssec/domain.com", mockRequester.calledUri)
	assert.Equal(t, "GET", mockRequester.calledMethod)
	assert.Equal(t, "", mockRequester.calledBodyData)
	assert.Equal(t, "domain.com", domain.DomainName)

	assert.Equal(t, 2015, domain.NextRollDate.Year())
	assert.Equal(t, "June", domain.NextRollDate.Month().String())
	assert.Equal(t, 13, domain.NextRollDate.Day())
	assert.Equal(t, 2, domain.NextRollDate.Hour())
	assert.Equal(t, 35, domain.NextRollDate.Minute())
	assert.Equal(t, 12, domain.NextRollDate.Second())
	assert.Equal(t, 543000000, domain.NextRollDate.Nanosecond())

	assert.Equal(t, []string{"record 1", "record 2"}, domain.DsRecords)
	assert.Equal(t, "publickey", domain.PublicSigningKey)
}

func TestGetDomainDnssecError(t *testing.T) {
	mockRequester := getMockRequester("", &NetkiError{"Error Message", make([]string, 0)})
	mockPartner := &NetkiPartner{Requester: mockRequester}

	domain, err := mockPartner.GetDomainDnssec(Domain{DomainName: "domain.com"})

	assert.NotEqual(t, nil, err)
	assert.NotEqual(t, nil, domain)
	assert.Equal(t, "Error Message", err.Error())
	assert.Equal(t, "/v1/partner/domain/dnssec/domain.com", mockRequester.calledUri)
	assert.Equal(t, "GET", mockRequester.calledMethod)
	assert.Equal(t, "", mockRequester.calledBodyData)
	assert.Equal(t, "", domain.DomainName)
}

func TestDeleteDomain(t *testing.T) {
	mockRequester := getMockRequester("", nil)
	mockPartner := &NetkiPartner{Requester: mockRequester}

	err := mockPartner.DeleteDomain(Domain{DomainName: "domain.com"})

	assert.Equal(t, nil, err)
}

func TestDeleteDomainError(t *testing.T) {
	mockRequester := getMockRequester("", &NetkiError{"Error Message", make([]string, 0)})
	mockPartner := &NetkiPartner{Requester: mockRequester}

	err := mockPartner.DeleteDomain(Domain{DomainName: "domain.com"})

	assert.NotEqual(t, nil, err)
	assert.Equal(t, "Error Message", err.Error())
}

func TestCreateNewWalletName(t *testing.T) {
	mockPartner := &NetkiPartner{}
	wn := mockPartner.CreateNewWalletName(Domain{DomainName: "domain.com"}, "walletname", make([]Wallet, 0), "externalId")

	assert.NotEqual(t, nil, wn)
	assert.Equal(t, "domain.com", wn.DomainName)
	assert.Equal(t, "walletname", wn.Name)
	assert.Equal(t, 0, len(wn.Wallets))
	assert.Equal(t, "externalId", wn.ExternalId)
}

func TestGetWalletNames(t *testing.T) {
	mockRequester := getMockRequester(`{"wallet_name_count":1,"wallet_names":[{"id":"id1","domain_name":"domain1.com","name":"name1","external_id":"ext1","wallets":[{"currency":"btc","wallet_address":"1btcaddress"}]}]}`, nil)
	mockPartner := &NetkiPartner{Requester: mockRequester}

	wns, err := mockPartner.GetWalletNames(Domain{DomainName: "domain.com"}, "ext1")

	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, wns)

	assert.Equal(t, "/v1/partner/walletname?domain_name=domain.com&external_id=ext1", mockRequester.calledUri)
	assert.Equal(t, "GET", mockRequester.calledMethod)
	assert.Equal(t, "", mockRequester.calledBodyData)

	assert.Equal(t, 1, len(wns))
	assert.Equal(t, "id1", wns[0].Id)
	assert.Equal(t, "domain1.com", wns[0].DomainName)
	assert.Equal(t, "name1", wns[0].Name)
	assert.Equal(t, "ext1", wns[0].ExternalId)
	assert.Equal(t, 1, len(wns[0].Wallets))
	assert.Equal(t, "btc", wns[0].Wallets[0].Currency)
	assert.Equal(t, "1btcaddress", wns[0].Wallets[0].WalletAddress)
}

func TestGetWalletNamesDomainOnly(t *testing.T) {
	mockRequester := getMockRequester(`{"wallet_name_count":1,"wallet_names":[{"id":"id1","domain_name":"domain1.com","name":"name1","external_id":"ext1","wallets":[{"currency":"btc","wallet_address":"1btcaddress"}]}]}`, nil)
	mockPartner := &NetkiPartner{Requester: mockRequester}

	wns, err := mockPartner.GetWalletNames(Domain{DomainName: "domain.com"}, "")

	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, wns)

	assert.Equal(t, "/v1/partner/walletname?domain_name=domain.com", mockRequester.calledUri)
	assert.Equal(t, "GET", mockRequester.calledMethod)
	assert.Equal(t, "", mockRequester.calledBodyData)

	assert.Equal(t, 1, len(wns))
	assert.Equal(t, "id1", wns[0].Id)
	assert.Equal(t, "domain1.com", wns[0].DomainName)
	assert.Equal(t, "name1", wns[0].Name)
	assert.Equal(t, "ext1", wns[0].ExternalId)
	assert.Equal(t, 1, len(wns[0].Wallets))
	assert.Equal(t, "btc", wns[0].Wallets[0].Currency)
	assert.Equal(t, "1btcaddress", wns[0].Wallets[0].WalletAddress)
}

func TestGetWalletNamesExtIdOnly(t *testing.T) {
	mockRequester := getMockRequester(`{"wallet_name_count":1,"wallet_names":[{"id":"id1","domain_name":"domain1.com","name":"name1","external_id":"ext1","wallets":[{"currency":"btc","wallet_address":"1btcaddress"}]}]}`, nil)
	mockPartner := &NetkiPartner{Requester: mockRequester}

	wns, err := mockPartner.GetWalletNames(Domain{}, "ext1")

	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, wns)

	assert.Equal(t, "/v1/partner/walletname?external_id=ext1", mockRequester.calledUri)
	assert.Equal(t, "GET", mockRequester.calledMethod)
	assert.Equal(t, "", mockRequester.calledBodyData)

	assert.Equal(t, 1, len(wns))
	assert.Equal(t, "id1", wns[0].Id)
	assert.Equal(t, "domain1.com", wns[0].DomainName)
	assert.Equal(t, "name1", wns[0].Name)
	assert.Equal(t, "ext1", wns[0].ExternalId)
	assert.Equal(t, 1, len(wns[0].Wallets))
	assert.Equal(t, "btc", wns[0].Wallets[0].Currency)
	assert.Equal(t, "1btcaddress", wns[0].Wallets[0].WalletAddress)
}

func TestGetWalletNamesEmptyArgs(t *testing.T) {
	mockRequester := getMockRequester(`{"wallet_name_count":1,"wallet_names":[{"id":"id1","domain_name":"domain1.com","name":"name1","external_id":"ext1","wallets":[{"currency":"btc","wallet_address":"1btcaddress"}]}]}`, nil)
	mockPartner := &NetkiPartner{Requester: mockRequester}

	wns, err := mockPartner.GetWalletNames(Domain{}, "")

	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, wns)

	assert.Equal(t, "/v1/partner/walletname", mockRequester.calledUri)
	assert.Equal(t, "GET", mockRequester.calledMethod)
	assert.Equal(t, "", mockRequester.calledBodyData)

	assert.Equal(t, 1, len(wns))
	assert.Equal(t, "id1", wns[0].Id)
	assert.Equal(t, "domain1.com", wns[0].DomainName)
	assert.Equal(t, "name1", wns[0].Name)
	assert.Equal(t, "ext1", wns[0].ExternalId)
	assert.Equal(t, 1, len(wns[0].Wallets))
	assert.Equal(t, "btc", wns[0].Wallets[0].Currency)
	assert.Equal(t, "1btcaddress", wns[0].Wallets[0].WalletAddress)
}

func TestGetWalletNamesError(t *testing.T) {
	mockRequester := getMockRequester("", &NetkiError{"Error Message", make([]string, 0)})
	mockPartner := &NetkiPartner{Requester: mockRequester}

	wns, err := mockPartner.GetWalletNames(Domain{}, "")

	assert.NotEqual(t, nil, err)
	assert.NotEqual(t, nil, wns)

	assert.Equal(t, "Error Message", err.Error())
	assert.Equal(t, "/v1/partner/walletname", mockRequester.calledUri)
	assert.Equal(t, "GET", mockRequester.calledMethod)
	assert.Equal(t, "", mockRequester.calledBodyData)

	assert.Equal(t, 0, len(wns))
}

func TestGetWalletNamesZeroWalletCount(t *testing.T) {
	mockRequester := getMockRequester(`{"wallet_name_count":0,"wallet_names":[]}]}`, nil)
	mockPartner := &NetkiPartner{Requester: mockRequester}

	wns, err := mockPartner.GetWalletNames(Domain{}, "")

	assert.Equal(t, nil, err)
	assert.NotEqual(t, nil, wns)

	assert.Equal(t, "/v1/partner/walletname", mockRequester.calledUri)
	assert.Equal(t, "GET", mockRequester.calledMethod)
	assert.Equal(t, "", mockRequester.calledBodyData)

	assert.Equal(t, 0, len(wns))
}

func TestWalletNameLookup(t *testing.T) {
	uri := "wallet.mattdavid.xyz"
	currency := "btc"
	s, err := WalletNameLookup(uri, currency)
	if err != nil {
		t.Error(err)
	}
	t.Log("Address:", s)
}

func TestWalletNameLookupBadname(t *testing.T) {
	uri := "badbad"
	currency := "btc"
	s, err := WalletNameLookup(uri, currency)
	if err == nil {
		t.Error("Got no error on bad currency")
	}
	t.Log("Address:", s)
}

func TestWalletNameLookupBadCurrency(t *testing.T) {
	uri := "wallet.mattdavid.xyz"
	currency := "badbad"
	s, err := WalletNameLookup(uri, currency)
	if err == nil {
		t.Error("Got no error on bad currency")
	}
	t.Log("Address:", s)
}

