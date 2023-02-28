<template>
  <q-btn @click="showCreateDialog = true" color="primary" icon="add" label="New Task Group"></q-btn>

  <q-dialog v-model="showCreateDialog">
    <ModifyTaskGroupCard style="width: 700px; max-width: 80vw;" @on-create="onCreate" />
  </q-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import ModifyTaskGroupCard from 'src/components/task-group/ModifyTaskGroupCard.vue'
import { TaskGroup } from 'src/stores/task-group-store'

const emit = defineEmits(['onCreate'])

function onCreate (project: TaskGroup) {
  showCreateDialog.value = false
  emit('onCreate', project)
}

const showCreateDialog = ref(false)
</script>
