package workers

import (
	"context"
	"log/slog"
	"time"

	"mytonstorage-backend/pkg/workers/cleaner"
	filesworker "mytonstorage-backend/pkg/workers/files"
)

type workerFunc = func(ctx context.Context) (interval time.Duration, err error)

type worker struct {
	files   filesworker.Worker
	cleaner cleaner.Worker
	logger  *slog.Logger
}

type Workers interface {
	Start(ctx context.Context) (err error)
}

func (w *worker) Start(ctx context.Context) (err error) {
	go w.run(ctx, "CleanupOldData", w.cleaner.CleanupOldData)

	go w.run(ctx, "MarkToRemoveUnpaidFiles", w.files.MarkToRemoveUnpaidFiles)
	go w.run(ctx, "RemoveUnpaidFiles", w.files.RemoveUnpaidFiles)

	/*
		Note: Первым отрабатывает CollectContractProvidersToNotify. Он дергает гет методы новых контрактов что бы получить список провайдеров
		Если удалось получить список провайдеров, то выставляет files.bag_users.notify_attempts = -1 и добавляет запись в providers.notifications
		Если возникла ошибка инкрементит счетчик попыток и возвращается в других итерациях

		Далее TriggerProvidersDownload собирает записи из providers.notifications
		В случае ошибки та же логика с попытками инкремента ошибок
		Если провайдер ответил что начал скачивание, то выставляет providers.notifications.notified = true

		Третьим идет DownloadChecker, который проверяет статус скачивания у провайдеров
		Используется тот же метод для проверки статуса что и в TriggerProvidersDownload
		В случае успеха обновляет количество скачанных байт в providers.notifications.downloaded = c.downloaded

		RemoveNotifiedFiles удаляет файлы
		Старше paidFilesLifetime часов
		И
		(
		Либо файлы провалившие TriggerProvidersDownload более N раз
		Либо файлы провалившие DownloadChecker более N раз
		Либо файлы которые полностью скачаны (downloaded = size)
		)
	*/
	go w.run(ctx, "CollectContractProvidersToNotify", w.files.CollectContractProvidersToNotify)
	go w.run(ctx, "TriggerProvidersDownload", w.files.TriggerProvidersDownload)
	go w.run(ctx, "DownloadChecker", w.files.DownloadChecker)
	go w.run(ctx, "RemoveNotifiedFiles", w.files.RemoveNotifiedFiles)

	return nil
}

func (w *worker) run(ctx context.Context, name string, f workerFunc) {
	logger := w.logger.With(slog.String("run_worker", name))

	for {
		select {
		case <-ctx.Done():
			return
		default:
			interval, err := f(ctx)
			if err != nil {
				logger.Error(err.Error())
			}
			if interval <= 0 {
				interval = time.Second
			}
			t := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}
		}
	}
}

func NewWorkers(
	files filesworker.Worker,
	cleaner cleaner.Worker,
	logger *slog.Logger,
) Workers {
	return &worker{
		files:   files,
		cleaner: cleaner,
		logger:  logger,
	}
}
