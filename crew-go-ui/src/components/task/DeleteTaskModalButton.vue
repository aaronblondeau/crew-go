<template>
  <q-btn @click="showDeleteDialog = true" :size="props.size" :flat="props.flat" color="red" icon="delete" :label="props.label"></q-btn>

  <q-dialog v-model="showDeleteDialog">
    <DeleteTaskCard :task="props.task" style="width: 700px; max-width: 80vw;" @on-delete="onDelete" @on-cancel="onCancel" />
  </q-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import DeleteTaskCard from 'src/components/task/DeleteTaskCard.vue'
import { Task } from 'src/stores/task-store'

export interface Props {
  flat?: boolean
  label?: string
  size?: string
  task: Task
}
const props = withDefaults(defineProps<Props>(), { label: 'Delete Task', flat: false, size: 'md' })

const emit = defineEmits(['onDelete'])

function onCancel () {
  showDeleteDialog.value = false
}

function onDelete () {
  showDeleteDialog.value = false
  emit('onDelete')
}

const showDeleteDialog = ref(false)
</script>
