package importer

import (
	"go_audio_search_api_server/globalTypes"
	"log/slog"
)

func (w *Worker) startPersistFilePool(workerAmount uint) {
	w.startPool(
		workerAmount,
		"Persist file pool",
		w.persistFileBuffer,
		w.persistFile,
		"Error persisting file",
	)
}

func (w *Worker) startTranscriptAudioPool(workerAmount uint) {
	w.startPool(
		workerAmount,
		"Transcript audio pool",
		w.transcriptAudioBuffer,
		w.transcribeAudio,
		"Error transcribing audio",
	)
}

func (w *Worker) startCreateEmbeddingsPool(workerAmount uint) {
	w.startPool(
		workerAmount,
		"Create embeddings pool",
		w.createEmbeddingsBuffer,
		w.createEmbeddings,
		"Error creating embeddings",
	)
}

func (w *Worker) startGenerateAiDataPool(workerAmount uint) {
	w.startPool(
		workerAmount,
		"Generate AI data pool",
		w.genAiDataBuffer,
		w.generateAiData,
		"Error generating AI data",
	)
}

func (w *Worker) startPool(
	workerAmount uint,
	poolName string,
	buffer <-chan *globalTypes.AudioDataElement,
	handler func(workerIdx uint, audioDataElement *globalTypes.AudioDataElement) error,
	errorMsg string,
) {
	for workerIdx := uint(0); workerIdx < workerAmount; workerIdx++ {
		idx := workerIdx

		w.WorkerWG.Add(1)
		go func() {
			defer w.WorkerWG.Done()

			for {
				select {
				case <-w.StopCtx.Done():
					slog.Info(poolName+" stopping ", "workerIdx", idx)
					return

				case audioDataElement := <-buffer:
					if err := handler(idx, audioDataElement); err != nil {
						slog.Error(errorMsg, "workerIdx", idx, "err", err)
					}
					w.PoolRefillSignal.Trigger()
				}
			}
		}()
	}
}
