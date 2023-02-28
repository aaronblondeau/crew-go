<template>
  <div v-if="!taskGroup" class="text-center q-ma-lg">
    <q-spinner
      color="primary"
      size="3em"
    />
  </div>
  <div v-if="taskGroup">

    <q-toolbar class="bg-indigo-6 text-white">
      <q-space />
      <q-breadcrumbs active-color="white" style="font-size: 16px">
        <q-breadcrumbs-el label="Home" icon="home" :to="{ name: 'home' }" />
        <q-breadcrumbs-el :label="taskGroup.name" icon="groups" :to="{ name: 'task_group', params: { taskGroupId: taskGroup.id }, query: { tab: 'tasks' } }" />
      </q-breadcrumbs>
      <q-space />
    </q-toolbar>
    <q-toolbar class="bg-indigo-6 text-white">
      <q-space />
      <!-- Wide Screen Tabs -->
      <q-tabs
        v-model="tab"
        inline-label
        mobile-arrows
        shrink
        class="text-white gt-xs"
      >
        <q-tab name="tasks" icon="checklist" label="Tasks" />
        <q-tab name="settings" icon="checklist" label="Settings" />
      </q-tabs>

      <!-- Narrow Screen Tabs -->
      <q-tabs
        v-model="tab"
        inline-label
        mobile-arrows
        shrink
        class="text-white lt-sm"
      >
        <q-tab name="tasks" icon="checklist" />
        <q-tab name="settings" icon="settings" />
      </q-tabs>
      <q-space />
    </q-toolbar>

    <q-tab-panels v-model="tab" animated>
      <q-tab-panel name="tasks">
        TODO - Tasks
      </q-tab-panel>
      <q-tab-panel name="settings">
        <ModifyTaskGroupCard :taskGroup="taskGroup" :closable="false" @on-save="onSave" />
        <div class="q-mt-md">
          <DeleteTaskGroupModalButton :taskGroup="taskGroup" @on-delete="onDelete" />
        </div>
      </q-tab-panel>
    </q-tab-panels>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { ref, watch, onMounted } from 'vue'
import { useTaskGroupStore, TaskGroup } from 'src/stores/task-group-store.js'
import notifyError from 'src/lib/notifyError'
import ModifyTaskGroupCard from 'src/components/task-group/ModifyTaskGroupCard.vue'
import DeleteTaskGroupModalButton from 'src/components/task-group/DeleteTaskGroupModalButton.vue'

const router = useRouter()

const taskGroupId = ref(router.currentRoute.value.params.taskGroupId as string)
const taskGroupStore = useTaskGroupStore()

const tab = ref(router.currentRoute.value.query.tab as string || 'tasks')
const taskGroup = ref<TaskGroup | null>(null)

async function getTaskGroup () {
  try {
    if (taskGroupId.value) {
      taskGroup.value = await taskGroupStore.getTaskGroup(taskGroupId.value)
    }
  } catch (e) {
    notifyError(e)
  }
}

function onSave (updatedTaskGroup: TaskGroup) {
  taskGroup.value = updatedTaskGroup
}

function onDelete () {
  router.replace({ name: 'home' })
}

watch(
  () => router.currentRoute.value,
  (newRoute) => {
    taskGroupId.value = newRoute.params.taskGroupId as string
  }
)

watch(
  () => taskGroupId.value,
  () => {
    getTaskGroup()
  }
)

watch(
  () => tab.value,
  (newTab) => {
    router.push({ query: { tab: newTab } })
  }
)

onMounted(() => {
  getTaskGroup()
})
</script>
