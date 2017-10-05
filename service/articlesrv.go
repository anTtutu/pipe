// Solo.go - A small and beautiful blogging platform written in golang.
// Copyright (C) 2017, b3log.org
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// Package service is the "business logic" layer, encapsulates transaction.
package service

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"

	"github.com/b3log/solo.go/model"
	"github.com/b3log/solo.go/util"
)

var Article = &articleService{
	mutex: &sync.Mutex{},
}

type articleService struct {
	mutex *sync.Mutex
}

// Article pagination arguments of admin console.
const (
	adminConsoleArticleListPageSize    = 15
	adminConsoleArticleListWindowsSize = 20
)

func (srv *articleService) ConsoleAddArticle(article *model.Article) error {
	srv.mutex.Lock()
	defer srv.mutex.Unlock()

	tx := db.Begin()
	if err := tx.Create(article).Error; nil != err {
		tx.Rollback()

		return err
	}
	tx.Commit()

	return nil
}

func (srv *articleService) ConsoleGetArticles(page int, blogID uint) (ret []*model.Article, pagination *util.Pagination) {
	offset := (page - 1) * adminConsoleArticleListPageSize
	count := 0
	db.Model(model.Article{}).Select("id, created_at, author_id, title, tags, path, topped, view_count, comment_count").
		Where(model.Article{Status: model.ArticleStatusPublished, BlogID: blogID}).
		Order("topped DESC, id DESC").Count(&count).
		Offset(offset).Limit(adminConsoleArticleListPageSize).
		Find(&ret)

	pageCount := int(math.Ceil(float64(count) / adminConsoleArticleListPageSize))
	pagination = util.NewPagination(page, adminConsoleArticleListPageSize, pageCount, adminConsoleArticleListWindowsSize, count)

	return
}

func (srv *articleService) ConsoleGetArticle(id uint) *model.Article {
	ret := &model.Article{}
	if nil != db.First(ret, id).Error {
		return nil
	}

	return ret
}

func (srv *articleService) ConsoleRemoveArticle(id uint) error {
	srv.mutex.Lock()
	defer srv.mutex.Unlock()

	article := &model.Article{}

	tx := db.Begin()
	if err := db.First(article, id).Error; nil != err {
		return err
	}
	author := &model.User{}
	if err := db.First(author, article.AuthorID).Error; nil != err {
		return err
	}
	author.ArticleCount = author.ArticleCount - 1
	if err := db.Model(&model.User{}).Updates(author).Error; nil != err {
		tx.Rollback()

		return err
	}
	if err := db.Delete(article).Error; nil != err {
		tx.Rollback()

		return err
	}
	if err := Statistic.DecArticleCountWithoutTx(author.BlogID); nil != err {
		tx.Rollback()

		return err
	}
	comments := []*model.Comment{}
	if err := db.Model(&model.Comment{}).Where("article_id = ?", id).
		Find(&comments).Error; nil != err {
		tx.Rollback()

		return err
	}
	if 0 < len(comments) {
		if err := db.Where("article_id = ?", id).Delete(&model.Comment{}).Error; nil != err {
			tx.Rollback()

			return err
		}

	}
	tx.Commit()

	return nil
}

func (srv *articleService) ConsoleUpdateArticle(article *model.Article) error {
	srv.mutex.Lock()
	defer srv.mutex.Unlock()

	count := 0
	if db.Model(&model.Article{}).Where("id = ?", article.ID).Count(&count); 1 > count {
		return errors.New(fmt.Sprintf("not found article [id=%d] to update", article.ID))
	}

	tx := db.Begin()
	if err := db.Model(&model.Article{}).Updates(article).Error; nil != err {
		tx.Rollback()

		return err
	}
	tx.Commit()

	return nil
}

func normalizeTagStr(tagStr string) string {
	reg := regexp.MustCompile(`\s+`)
	tagStr = reg.ReplaceAllString(tagStr, "")
	tagStr = strings.Replace(tagStr, "，", ",", -1)
	tagStr = strings.Replace(tagStr, "、", ",", -1)
	tagStr = strings.Replace(tagStr, "；", ",", -1)
	tagStr = strings.Replace(tagStr, ";", ",", -1)

	reg = regexp.MustCompile(`[\u4e00-\u9fa5,\w,&,\+,\-,\.]+`)
	tags := strings.Split(tagStr, ",")
	retTags := []string{}
	for _, tag := range tags {
		if contains(retTags, tag) {
			continue
		}

		if !reg.MatchString(tag) {
			continue
		}

		retTags = append(retTags, tag)
	}

	return strings.Join(retTags, ",")
}

func contains(strs []string, str string) bool {
	for _, s := range strs {
		if s == str {
			return true
		}
	}

	return false
}
