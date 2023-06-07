<template>
  <q-btn @click="showResetDialog = true" size="md" flat color="orange" icon="skip_previous">
    <q-tooltip>
      Reset this task
    </q-tooltip>
  </q-btn>

  <q-dialog v-model="showResetDialog">
    <q-card>
      <q-card-section class="row items-center q-pb-none">
        <div class="text-h6">
          Reset Task?
        </div>
        <q-space />
        <q-btn icon="close" flat round dense v-close-popup />
      </q-card-section>

      <q-card-section>
        This task will be set to its original state and will be available to be run again. All output will be lost.
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
          @click="reset"
          color="orange"
          class="full-width q-mt-md"
          :loading="resetWait"
          :disable="resetWait"
          label="Reset"
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

const resetWait = ref(false)
const emit = defineEmits(['onReset'])
const remainingAttempts = ref(5)

async function reset () {
  try {
    resetWait.value = true
    const updatedTask = await taskStore.resetTask(props.task.taskGroupId, props.task.id, remainingAttempts.value)
    showResetDialog.value = false
    emit('onReset', updatedTask)
  } finally {
    resetWait.value = false
  }
}

const showResetDialog = ref(false)
</script>
