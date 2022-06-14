module github.com/emersion/go-smtp-proxy

go 1.13

require (
	github.com/emersion/go-sasl v0.0.0-20200509203442-7bfe0ed36a21
	github.com/emersion/go-smtp v0.12.0
)

replace github.com/emersion/go-smtp => github.com/kayrus/go-smtp v0.15.1-0.20220614174538-08dbc71e586d
