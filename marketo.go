package marketo

/*
	issues:
	    1. how long before the acces token expires and how to deal with
*/

import (
	"fmt"
	"net/http"
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"time"
	"strings"
)

type Token struct {
	Access_token string `json:"access_token"`
	Token_type   string `json:"token_type"`
	Expires_in   int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

type Response struct {
	Success bool `json:"success"`
}

type Lead struct {
	Id                   int  `json:"id"`
	Email                string `json:"email"`
	SecurityCheckerKey   string `json:"securityCheckerKey"`
	SCReportURL          string `json:"sCReportURL"`
	
}

type LeadQueryResponse struct {
	RequestId  string `json:"requestId"`
	Result     []*Lead `json:"result"`
	Success    bool   `json:"success"`
	
}

type Marketo struct {
	MarketoInstance string
	ClientId        string
	ClientSecret    string
	client          *http.Client
	Log             *log.Logger
}

func (m *Marketo) getToken() (*Token, bool, error) {
	t0 := time.Now()
	if m.client == nil {
		m.client = &http.Client{}
	}
	buf := new(bytes.Buffer)
	var token Token

	//request_url :=  url.QueryEscape(  fmt.Sprintf("%s&client_id=%s&client_secret=%s", m.MarketoInstance, m.ClientId, m.ClientSecret) )
	request_url := fmt.Sprintf("%s/identity/oauth/token?grant_type=client_credentials&client_id=%s&client_secret=%s", m.MarketoInstance, m.ClientId, m.ClientSecret)

	resp, err := m.client.Get(request_url)
	if err != nil {
		log.Printf("ERROR getting token \n%s\n", err)
		return nil, false, err
	}
	defer resp.Body.Close()

	
	if resp.StatusCode != 200 {
		msg := fmt.Sprintf("get marketo token with response : %s", resp.Status)
		return nil , false, errors.New(msg)
	}

	if _, err := buf.ReadFrom(resp.Body); err != nil {
		fmt.Printf("ERROR reading response: \n%s\n", err)
		return nil, false, err
	}

	if err := json.Unmarshal([]byte(buf.String()), &token); err != nil {
		m.Log.Printf("ERROR unmashalling %s\n%s\n", buf.String(), err)
		return nil, false, err
	}
	
	
	t1 := time.Now()
	m.Log.Printf("getToken time duration : %v\n", t1.Sub(t0))

	return &token, true, nil
}

func (m *Marketo) UpdateReportUrl(email string, reportUrl string, scanNumber int) (bool, error) {
	t0 := time.Now()
	token, ok, err := m.getToken()
	if !ok {
		m.Log.Printf("ERROR getting token %s\n", err)
		return false, err
	}
	
	url := fmt.Sprintf("%s/rest/v1/leads.json?access_token=%s", m.MarketoInstance, token.Access_token)
	m.Log.Printf("url : \n%s\n\n", url)

	jsonStr := []byte(fmt.Sprintf("{\"input\":[{\"email\":\"%s\",\"sCReportURL%d\":\"%s\"}],\"action\":\"createOrUpdate\"}", email,scanNumber, reportUrl))
	m.Log.Printf("jsonString :\n%s\n\n", jsonStr)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
	if err != nil {
		m.Log.Printf("ERROR creating request : \n%s\n", err)
		return false, err
	}
	req.Header.Set("Content-type", "application/json")
	req.Header.Set("accept", "text/json")

	resp, err := m.client.Do(req)
	if err != nil {
		m.Log.Printf("ERROR doing post on marketo in UpdateReportUrl : \n%s\n", err)
		return false, err
	}
	defer resp.Body.Close()

	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		m.Log.Printf("ERROR in UpdateReportUrl in readall : \n%s\n", err)
		return false, nil
	}

	fmt.Printf("body : \n%s\n", string(body))
	
	t1 := time.Now()
	m.Log.Printf("UpdateReportURL time duration : %v\n", t1.Sub(t0))

	return true, nil
}

func (m *Marketo) isSecurityCheckerKeyValid(email string, securityCheckerKey string) (bool, error) {
	lead, err := m.getMarketoLead(email)
	if err != nil {
		return false, err
	}
	if strings.Compare(securityCheckerKey, lead.SecurityCheckerKey) == 0 {
		return true , nil
	}else {
		return false, nil
	}
	
	
}

func (m *Marketo) getMarketoLead(email string) (lead *Lead, err error) {
	t0 := time.Now()

	filterType := "email"
	filterValues := email
	fields := "securityCheckerKey,email,sCReportURL,sCReportURL1,sCReportURL2,sCReportURL3,sCReportURL4"
	buf := new(bytes.Buffer)
	var response LeadQueryResponse
	
	token, ok, err := m.getToken()
	if !ok {
		m.Log.Printf("ERROR getting token %s\n", err)
		return nil , err
	}

	url := fmt.Sprintf("%s/rest/v1/leads.json?access_token=%s&filterType=%s&filterValues=%s&fields=%s&_method=GET", m.MarketoInstance, token.Access_token, filterType, filterValues, fields)
	
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		m.Log.Printf("ERROR creating request in getMarketoId : \n%s\n", err)
		return nil, err
	}
	req.Header.Set("accept", "text/json")

	resp, err := m.client.Do(req)
	if err != nil {
		m.Log.Printf("ERROR doing post on marketo in getMarketoId : \n%s\n", err)
		return nil, err
	}
	defer resp.Body.Close()

	
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		m.Log.Printf("ERROR reading response: \n%s\n", err)
		return nil,  err
	}
	
	if err := json.Unmarshal([]byte(buf.String()), &response); err != nil {
		m.Log.Printf("ERROR unmarshalling %s\n%s\n%s\n", buf.String(), err)
		return nil,  err
	}
	
	if !response.Success {
		m.Log.Printf("ERROR %s\n", buf.String())
		return nil , errors.New(buf.String())
	}
	
	
	// check if there are results
	if len(response.Result) != 1 {
		m.Log.Printf("ERROR zero or several leads found for %s\n%s\n", email, buf.String())
		return nil, errors.New(buf.String())
	}	

	t1 := time.Now()
	m.Log.Printf("getMarketoLead time duration : %v\n", t1.Sub(t0))
	

	return response.Result[0], nil
}
