<template>
  <q-btn v-if="!task.isComplete && task.remainingAttempts <= 0" @click="showRetryDialog = true" size="md" flat color="purple" icon="restart_alt">
    <q-tooltip>
      Retry this task
    </q-tooltip>
  </q-btn>

  <q-dialog v-model="showRetryDialog">
    <q-card>
      <q-card-section class="row items-center q-pb-none">
        <div class="text-h6">
          Retry Task?
        </div>
        <q-space />
        <q-btn icon="close" flat round dense v-close-popup />
      </q-card-section>

      <q-card-section>
        <q-input
          v-model="remainingAttempts"
          label="Task Remaining Attempts"
          type="number"
          filled
          />
      </q-card-section>

      <q-card-actions>
        <q-btn
          @click="retry"
          color="purple"
          class="full-width q-mt-md"
          :loading="retryWait"
          :disable="retryWait"
          label="Retry"
          />
      </q-card-actions>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useTaskStore, Task } from 'src/stores/task-store'

export interface Props {
  task: Task
}
const props = defineProps<Props>()

const taskStore = useTaskStore()

const retryWait = ref(false)
const emit = defineEmits(['onRetry'])
const remainingAttempts = ref(5)

async function retry () {
  try {
    retryWait.value = true
    const updatedTask = await taskStore.retryTask(props.task.taskGroupId, props.task.id, remainingAttempts.value)
    showRetryDialog.value = false
    emit('onRetry', updatedTask)
  } finally {
    retryWait.value = false
  }
}

const showRetryDialog = ref(false)
</script>
