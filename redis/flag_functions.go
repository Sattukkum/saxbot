package redis

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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

func RestoreBackup() {
	backupPath := filepath.Join("redis_data", "dump.rdb")

	fmt.Printf("Восстанавливаем базу данных Redis из бэкапа...\n")

	// Проверяем существование файла бэкапа
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		log.Fatalf("Файл бэкапа %s не найден", backupPath)
	}

	// Показываем что было до восстановления
	keys, err := GetAllKeys()
	if err == nil {
		fmt.Printf("Ключей до восстановления: %d\n", len(keys))
	}

	// Очищаем текущую базу данных
	fmt.Printf("Очищаем текущую базу данных...\n")
	err = FlushAll()
	if err != nil {
		log.Fatalf("Ошибка очистки Redis перед восстановлением: %v", err)
	}

	// Закрываем соединение перед восстановлением
	fmt.Printf("Закрываем соединение с Redis...\n")
	err = CloseRedis()
	if err != nil {
		log.Printf("Предупреждение: ошибка закрытия соединения: %v", err)
	}

	// Используем redis-cli для загрузки дампа
	fmt.Printf("Загружаем данные из %s...\n", backupPath)

	// Получаем абсолютный путь к файлу дампа
	absBackupPath, err := filepath.Abs(backupPath)
	if err != nil {
		log.Fatalf("Ошибка получения абсолютного пути: %v", err)
	}

	// Команда для восстановления через redis-cli
	cmd := exec.Command("redis-cli", "--rdb", absBackupPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Пробуем альтернативный способ - копирование файла в директорию Redis
		fmt.Printf("Прямое восстановление не удалось (%v), пробуем альтернативный способ...\n", err)

		// Для альтернативного способа нужно остановить Redis, скопировать файл и запустить заново
		fmt.Printf("ВНИМАНИЕ: Для восстановления бэкапа выполните следующие шаги вручную:\n")
		fmt.Printf("1. Остановите Redis сервер\n")
		fmt.Printf("2. Скопируйте файл %s в директорию данных Redis (обычно /usr/local/var/db/redis/ или /var/lib/redis/)\n", absBackupPath)
		fmt.Printf("3. Переименуйте скопированный файл в dump.rdb\n")
		fmt.Printf("4. Запустите Redis сервер заново\n")
		fmt.Printf("5. Запустите программу без флага --restore-backup\n")
		return
	}

	fmt.Printf("Вывод redis-cli: %s\n", string(output))

	// Переинициализируем соединение для проверки
	fmt.Printf("Переподключаемся к Redis...\n")
	err = InitRedis("localhost:6379", "", 0)
	if err != nil {
		log.Printf("Предупреждение: не удалось переподключиться для проверки: %v", err)
		fmt.Printf("Восстановление завершено. Перезапустите программу для проверки результата.\n")
		return
	}

	// Проверяем результат
	keys, err = GetAllKeys()
	if err == nil {
		fmt.Printf("Ключей после восстановления: %d\n", len(keys))
		if len(keys) > 0 {
			fmt.Printf("Восстановление завершено успешно!\n")
		} else {
			fmt.Printf("Предупреждение: база данных пуста после восстановления\n")
		}
	} else {
		fmt.Printf("Не удалось проверить результат восстановления: %v\n", err)
	}
}
