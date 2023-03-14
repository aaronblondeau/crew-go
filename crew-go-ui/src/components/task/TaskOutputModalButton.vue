<template>
  <q-btn @click="showOutputDialog = true" size="md" flat :color="props.task.errors.length > 0 ? 'orange' : 'primary'" :icon="props.task.errors.length > 0 ? 'report_problem' : 'info'">
    <q-tooltip>
      Task Output / Errors
    </q-tooltip>
  </q-btn>

  <q-dialog v-model="showOutputDialog">
    <q-card>
      <q-card-section class="row items-center q-pb-none">
        <div class="text-h6">
          Task Output
        </div>
        <q-space />
        <q-btn icon="close" flat round dense v-close-popup />
      </q-card-section>

      <q-card-section>
        {{ props.task.output }}}
      </q-card-section>

      <q-card-section v-if="props.task.errors.length > 0">
        <div class="text-h6">
          Task Errors
        </div>
      </q-card-section>

      <q-card-section v-if="props.task.errors.length > 0">
        {{ props.task.errors }}
      </q-card-section>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { Task } from 'src/stores/task-store'

export interface Props {
  task: Task
}
const props = defineProps<Props>()
const showOutputDialog = ref(false)
</script>
