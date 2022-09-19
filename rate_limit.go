package main

import "time"

const Unlimited = -1 // без ограничений
const NotUsed = -2   // при выборе констант следует учитывать, что в коде есть провеки limit < notUsed => Invalid Value

type RateLimiter interface {
	/*
		На каком интервале работает лимитер. Например, 1 сек или 1 мин.
		Период задается при создании лимитера и не может измениться в процессе работы.
	*/
	LimitPeriod() time.Duration

	/*
		Проверяет возможность вызова метода.
		sellerId:	должен передаваться всегда
		group:		опциональный параметр, если в рамках селлера ведется учет по группам

		allow:		вызов разрешен/нет
		limit:		общий лимит. >= 0 или Unlimited
		remain:		сколько еще вызовов можно сделать в рамках текущего периода. >= 0 или Unlimited
	*/
	Check(sellerId string, group string) (allow bool, limit int, remain int)
	/*
		Удаляет просроченные лимиты.
		Если maxRecords > 0, то за раз удалит не более указанного количества.
	*/
	DeleteExpired(maxRecords int) (nbRecordsDeleted int)
}

type IConfigurableRateLimiter interface {
	RateLimiter
	/*
		Устанавливает новые группы учета.
		Предыдущие удаляются
		Можно передать nil/пустой массив, чтобы убрать все группы учета
		Все ранее сконфиженные лимиты кроме baseLimit удаляются
	*/
	SetAccountingGroups(groups []string)
	/*
		seller:	для кого настраивается лимит. Если "", то устанавливаются глобальные лимиты
		group:	дял какой группы устанавливается лимит. Если "", то устанавливается базовый лимит.
		limit:	можно передать NotUsed, тогда указанный лимит будет удален, в остальных случаях он будет обновлен
	*/
	SetLimit(seller, group string, limit int) error
	/*
		Удаляет все перегрузки для указанного селлера (то есть все ключи seller|*)
		Если seller="", то удаляет все перегрузки всех селлеров, оставляя только baseLimit + базовые димиты по группам
	*/
	RemoveSellerLimits(seller string)
	RemoveAllOverrides() // удалит все перегрузки лимитов, оставляя только baseLimit
}
