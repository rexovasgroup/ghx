package api

// Organization represents a GitHub organization with a login name.
type Organization struct {
	Login string
}

// CurrentLoginName returns the login name of the currently authenticated user.
func CurrentLoginName(client *Client, hostname string) (string, error) {
	var query struct {
		Viewer struct {
			Login string
		}
	}
	err := client.Query(hostname, "UserCurrent", &query, nil)
	return query.Viewer.Login, err
}

// CurrentLoginNameAndOrgs returns the login name and organization memberships of the currently authenticated user.
func CurrentLoginNameAndOrgs(client *Client, hostname string) (string, []string, error) {
	var query struct {
		Viewer struct {
			Login         string
			Organizations struct {
				Nodes []Organization
			} `graphql:"organizations(first: 100)"`
		}
	}
	err := client.Query(hostname, "UserCurrent", &query, nil)
	if err != nil {
		return "", nil, err
	}
	orgNames := []string{}
	for _, org := range query.Viewer.Organizations.Nodes {
		orgNames = append(orgNames, org.Login)
	}
	return query.Viewer.Login, orgNames, nil
}

// CurrentUserID returns the node ID of the currently authenticated user.
func CurrentUserID(client *Client, hostname string) (string, error) {
	var query struct {
		Viewer struct {
			ID string
		}
	}
	err := client.Query(hostname, "UserCurrent", &query, nil)
	return query.Viewer.ID, err
}
