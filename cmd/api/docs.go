// Package main is the entry point of the ErrHub backend.
//
//	@title          ErrHub API
//	@version        1.0
//	@description    Company-wide error tracking hub. Receives error events from the errhub-package SDK, aggregates them into issues and exposes a REST API for the dashboard frontend.
//	@contact.name   ErrHub Support
//
//	@host       localhost:8080
//	@BasePath   /
//	@schemes    http https
//
//	@securityDefinitions.apikey BearerAuth
//	@in                         header
//	@name                       Authorization
//	@description                JWT token — obtain via POST /api/auth/login. Format: "Bearer <token>"
package main
