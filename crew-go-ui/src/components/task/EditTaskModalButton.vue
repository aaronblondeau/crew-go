<template>
  <q-btn @click="showEditDialog = true" :size="props.size" :flat="props.flat" color="primary" icon="edit" :label="props.label"></q-btn>

  <q-dialog v-model="showEditDialog">
    <ModifyTaskCard :task-group="props.taskGroup" :task="task" style="width: 700px; max-width: 80vw;" @on-update="onUpdate" />
  </q-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import ModifyTaskCard from 'src/components/task/ModifyTaskCard.vue'
import { Task } from 'src/stores/task-store'
import { TaskGroup } from 'src/stores/task-group-store'

export interface Props {
  flat?: boolean
  label?: string
  size?: string
  task: Task,
  taskGroup: TaskGroup
}
const props = withDefaults(defineProps<Props>(), { label: 'Edit Task', flat: false, size: 'md' })

const emit = defineEmits(['onUpdate'])

function onUpdate (task: Task) {
  showEditDialog.value = false
  emit('onUpdate', task)
}

const showEditDialog = ref(false)
</script>
