package redis

import (
	"fmt"
	"log"
)

func ShowInfo() {
	keys, err := GetAllKeys()
	if err != nil {
		log.Fatalf("Ошибка получения ключей: %v", err)
	}

	fmt.Printf("Информация о Redis базе данных:\n")
	fmt.Printf("Всего ключей: %d\n", len(keys))

	if len(keys) > 0 {
		fmt.Printf("Ключи:\n")
		for i, key := range keys {
			if i >= 10 { // Показываем только первые 10 ключей
				fmt.Printf("   ... и еще %d ключей\n", len(keys)-10)
				break
			}
			fmt.Printf("   - %s\n", key)
		}
	} else {
		fmt.Printf("База данных пуста\n")
	}
}

func ClearRedis() {
	fmt.Printf("Очищаем базу данных Redis...\n")

	// Показываем что было до очистки
	keys, err := GetAllKeys()
	if err == nil {
		fmt.Printf("Найдено ключей для удаления: %d\n", len(keys))
	}

	err = FlushAll()
	if err != nil {
		log.Fatalf("Ошибка очистки Redis: %v", err)
	}

	fmt.Printf("База данных Redis очищена!\n")

	// Проверяем что действительно очистилось
	keys, err = GetAllKeys()
	if err == nil {
		fmt.Printf("Ключей после очистки: %d\n", len(keys))
	}
}
