// Copyright 2021 The Casdoor Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package object

import (
	"fmt"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	xormadapter "github.com/casbin/xorm-adapter/v2"
	"github.com/casdoor/casdoor/conf"
	"github.com/casdoor/casdoor/util"
	"xorm.io/core"
)

type Permission struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	DisplayName string `xorm:"varchar(100)" json:"displayName"`

	Users []string `xorm:"mediumtext" json:"users"`
	Roles []string `xorm:"mediumtext" json:"roles"`

	ResourceType string   `xorm:"varchar(100)" json:"resourceType"`
	Resources    []string `xorm:"mediumtext" json:"resources"`
	Actions      []string `xorm:"mediumtext" json:"actions"`
	Effect       string   `xorm:"varchar(100)" json:"effect"`

	IsEnabled bool `json:"isEnabled"`
}

var Enforcer *casbin.Enforcer

func GetPermissionCount(owner, field, value string) int {
	session := GetSession(owner, -1, -1, field, value, "", "")
	count, err := session.Count(&Permission{})
	if err != nil {
		panic(err)
	}

	return int(count)
}

func GetPermissions(owner string) []*Permission {
	permissions := []*Permission{}
	err := adapter.Engine.Desc("created_time").Find(&permissions, &Permission{Owner: owner})
	if err != nil {
		panic(err)
	}

	return permissions
}

func GetPaginationPermissions(owner string, offset, limit int, field, value, sortField, sortOrder string) []*Permission {
	permissions := []*Permission{}
	session := GetSession(owner, offset, limit, field, value, sortField, sortOrder)
	err := session.Find(&permissions)
	if err != nil {
		panic(err)
	}

	return permissions
}

func getPermission(owner string, name string) *Permission {
	if owner == "" || name == "" {
		return nil
	}

	permission := Permission{Owner: owner, Name: name}
	existed, err := adapter.Engine.Get(&permission)
	if err != nil {
		panic(err)
	}

	if existed {
		return &permission
	} else {
		return nil
	}
}

func GetPermission(id string) *Permission {
	owner, name := util.GetOwnerAndNameFromId(id)
	return getPermission(owner, name)
}

func UpdatePermission(id string, permission *Permission) bool {
	owner, name := util.GetOwnerAndNameFromId(id)
	if getPermission(owner, name) == nil {
		return false
	}

	affected, err := adapter.Engine.ID(core.PK{owner, name}).AllCols().Update(permission)
	if err != nil {
		panic(err)
	}

	if affected != 0 {
		updatePolicies(permission)
	}

	return affected != 0
}

func AddPermission(permission *Permission) bool {
	affected, err := adapter.Engine.Insert(permission)
	if err != nil {
		panic(err)
	}

	if affected != 0 {
		updatePolicies(permission)
	}

	return affected != 0
}

func DeletePermission(permission *Permission) bool {
	affected, err := adapter.Engine.ID(core.PK{permission.Owner, permission.Name}).Delete(&Permission{})
	if err != nil {
		panic(err)
	}

	return affected != 0
}

func (permission *Permission) GetId() string {
	return fmt.Sprintf("%s/%s", permission.Owner, permission.Name)
}

func InitPermission() {
	var err error

	tableNamePrefix := conf.GetConfigString("tableNamePrefix")
	adapter, err := xormadapter.NewAdapterWithTableName(conf.GetConfigString("driverName"), conf.GetBeegoConfDataSourceName()+conf.GetConfigString("dbName"), "permission_rule", tableNamePrefix, true)
	if err != nil {
		panic(err)
	}

	modelText := `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act, eft, id

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow)) && !some(where (p.eft == deny))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
`
	m, err := model.NewModelFromString(modelText)
	if err != nil {
		panic(err)
	}

	Enforcer, err = casbin.NewEnforcer(m, adapter)
	if err != nil {
		panic(err)
	}

	err = Enforcer.LoadPolicy()
	if err != nil {
		panic(err)
	}
}

	func updatePolicies(permission *Permission) {
	id := fmt.Sprintf("%s/%s", permission.Owner, permission.Name)

	var policies [][]string
	for _, user := range permission.Users {
		for _, resource := range permission.Resources {
			for _, action := range permission.Actions {
				policies = append(policies, []string{user, resource, strings.ToLower(action), strings.ToLower(permission.Effect), id})
			}
		}
	}
	for _, role := range permission.Roles {
		for _, resource := range permission.Resources {
			for _, action := range permission.Actions {
				policies = append(policies, []string{role, resource, strings.ToLower(action), strings.ToLower(permission.Effect), id})
			}
		}
	}

	_, err := Enforcer.RemoveFilteredPolicy(4, id)
	if err != nil {
		panic(err)
	}
	_, err = Enforcer.AddPolicies(policies)
	if err != nil {
		panic(err)
	}
}