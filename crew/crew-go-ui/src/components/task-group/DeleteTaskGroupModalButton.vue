<template>
  <q-btn @click="showDeleteDialog = true" color="red" icon="delete" label="Delete Task Group"></q-btn>

  <q-dialog v-model="showDeleteDialog">
    <DeleteTaskGroupCard :taskGroup="props.taskGroup" style="width: 700px; max-width: 80vw;" @on-delete="onDelete" @on-cancel="onCancel" />
  </q-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import DeleteTaskGroupCard from 'src/components/task-group/DeleteTaskGroupCard.vue'
import { TaskGroup } from 'src/stores/task-group-store'

export interface Props {
  taskGroup: TaskGroup
}
const props = defineProps<Props>()

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
