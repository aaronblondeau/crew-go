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
        <q-tab name="graph" icon="account_tree" label="Graph View" />
        <q-tab name="settings" icon="settings" label="Settings" />
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
        <q-tab name="graph" icon="account_tree" />
        <q-tab name="settings" icon="settings" />
      </q-tabs>
      <q-space />
    </q-toolbar>

    <q-tab-panels v-model="tab" animated>
      <q-tab-panel name="tasks">

        <div class="q-pb-md">
          <q-linear-progress color="green" size="25px" :value="completedPercent">
            <div class="absolute-full flex flex-center">
              <q-badge color="white" text-color="accent" :label="(completedPercent * 100).toFixed(2) + '%'" />
            </div>
          </q-linear-progress>
        </div>

        <div class="q-gutter-md">
          <q-btn label="Reset" color="orange" @click="onInitReset" icon="skip_previous" />
          <q-btn label="Retry" color="purple" @click="onInitRetry" icon="restart_alt" />
          <q-btn label="Pause" color="primary" @click="onPause" icon="pause" />
          <q-btn label="Resume" color="primary" @click="onResume" icon="play_arrow" />
        </div>

        <TaskTable ref="taskTable" :task-group="taskGroup" class="q-mt-md" />
      </q-tab-panel>
      <q-tab-panel name="graph">
        <div class="q-pb-md">
          <q-linear-progress color="green" size="25px" :value="completedPercent">
            <div class="absolute-full flex flex-center">
              <q-badge color="white" text-color="accent" :label="(completedPercent * 100).toFixed(2) + '%'" />
            </div>
          </q-linear-progress>
        </div>

        <div class="q-gutter-md">
          <q-btn label="Reset" color="orange" @click="onInitReset" icon="skip_previous" />
          <q-btn label="Retry" color="purple" @click="onInitRetry" icon="restart_alt" />
          <q-btn label="Pause" color="primary" @click="onPause" icon="pause" />
          <q-btn label="Resume" color="primary" @click="onResume" icon="play_arrow" />
        </div>

        <TaskGraph ref="taskGraph" :task-group="taskGroup" class="q-mt-md" />
      </q-tab-panel>
      <q-tab-panel name="settings">
        <ModifyTaskGroupCard :taskGroup="taskGroup" :closable="false" @on-save="onSave" />
        <div class="q-mt-md">
          <DeleteTaskGroupModalButton :taskGroup="taskGroup" @on-delete="onDelete" />
        </div>
      </q-tab-panel>
    </q-tab-panels>

    <q-dialog v-model="showResetDialog">
      <q-card>
        <q-card-section class="row items-center q-pb-none">
          <div class="text-h6">
            Reset Task Group?
          </div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>

        <q-card-section>
          All tasks in the group will be reset to their initial state. If the group contains seed tasks, all non-seed tasks will be deleted.
        </q-card-section>

        <q-card-section>
          <q-input
            v-model="resetRemainingAttempts"
            label="Task Remaining Attempts"
            type="number"
            filled
            />
        </q-card-section>

        <q-card-actions>
          <q-btn
            @click="onReset"
            color="orange"
            class="full-width q-mt-md"
            :loading="resetWait"
            :disable="resetWait"
            label="Reset"
            />
        </q-card-actions>
      </q-card>
    </q-dialog>

    <q-dialog v-model="showRetryDialog">
      <q-card>
        <q-card-section class="row items-center q-pb-none">
          <div class="text-h6">
            Retry Task Group?
          </div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>

        <q-card-section>
          <q-input
            v-model="retryRemainingAttempts"
            label="Task Remaining Attempts"
            type="number"
            filled
            />
        </q-card-section>

        <q-card-actions>
          <q-btn
            @click="onRetry"
            color="purple"
            class="full-width q-mt-md"
            :loading="retryWait"
            :disable="retryWait"
            label="Retry"
            />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import _ from 'lodash'
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { useTaskGroupStore, TaskGroup } from 'src/stores/task-group-store.js'
import notifyError from 'src/lib/notifyError'
import ModifyTaskGroupCard from 'src/components/task-group/ModifyTaskGroupCard.vue'
import DeleteTaskGroupModalButton from 'src/components/task-group/DeleteTaskGroupModalButton.vue'
import TaskTable from 'src/components/task/TaskTable.vue'
import TaskGraph from 'src/components/task/TaskGraph.vue'
import { useQuasar } from 'quasar'

const router = useRouter()
const $q = useQuasar()

const taskGroupId = ref(router.currentRoute.value.params.taskGroupId as string)
const taskGroupStore = useTaskGroupStore()

const tab = ref(router.currentRoute.value.query.tab as string || 'tasks')
const taskGroup = ref<TaskGroup | null>(null)
const taskTable = ref<typeof TaskTable>()
const taskGraph = ref<typeof TaskGraph>()
const resetWait = ref(false)
const retryWait = ref(false)
const pauseWait = ref(false)
const resumeWait = ref(false)
const showResetDialog = ref(false)
const resetRemainingAttempts = ref(5)
const showRetryDialog = ref(false)
const retryRemainingAttempts = ref(5)
const completedPercent = ref(0.0)

async function getTaskGroup () {
  try {
    if (taskGroupId.value) {
      taskGroup.value = await taskGroupStore.getTaskGroup(taskGroupId.value)
      await updateCompletedPercent()
    }
  } catch (e) {
    notifyError(e)
  }
}

const throttledUpdateCompletedPercent = _.throttle(updateCompletedPercent, 5000)

async function updateCompletedPercent () {
  completedPercent.value = await taskGroupStore.getTaskGroupProgress(taskGroupId.value)
}

function onSave (updatedTaskGroup: TaskGroup) {
  taskGroup.value = updatedTaskGroup
}

function onDelete () {
  router.replace({ name: 'home' })
}

async function onInitReset () {
  showResetDialog.value = true
}

async function onReset () {
  try {
    resetWait.value = true
    await taskGroupStore.resetTaskGroup(taskGroupId.value, resetRemainingAttempts.value)
    await taskTable.value?.loadTasks()
    await taskGraph.value?.loadTasks()
    showResetDialog.value = false
  } catch (e) {
    notifyError(e)
  } finally {
    resetWait.value = false
  }
}

async function onInitRetry () {
  showRetryDialog.value = true
}

async function onRetry () {
  try {
    retryWait.value = true
    await taskGroupStore.retryTaskGroup(taskGroupId.value, retryRemainingAttempts.value)
    await taskTable.value?.loadTasks()
    await taskGraph.value?.loadTasks()
    showRetryDialog.value = false
  } catch (e) {
    notifyError(e)
  } finally {
    retryWait.value = false
  }
}

async function onPause () {
  try {
    pauseWait.value = true
    await taskGroupStore.pauseTaskGroup(taskGroupId.value)
    await taskTable.value?.loadTasks()
    await taskGraph.value?.loadTasks()
  } catch (e) {
    notifyError(e)
  } finally {
    pauseWait.value = false
  }
}

async function onResume () {
  try {
    resumeWait.value = true
    await taskGroupStore.resumeTaskGroup(taskGroupId.value)
    await taskTable.value?.loadTasks()
    await taskGraph.value?.loadTasks()
  } catch (e) {
    notifyError(e)
  } finally {
    resumeWait.value = false
  }
}

let cancelWatchGroup : null | (() => void) = null

function unwatchGroup () {
  if (cancelWatchGroup) {
    cancelWatchGroup()
  }
}

async function watchGroup () {
  unwatchGroup()
  cancelWatchGroup = await taskGroupStore.watchTaskGroup(taskGroupId.value, (event: any) => {
    const payload = JSON.parse(event)
    if (payload.type === 'update' && _.has(payload, 'task_group')) {
      taskGroup.value = payload.task_group
    } else if (payload.type === 'delete' && _.has(payload, 'task_group')) {
      // This task group has been deleted!
      $q.dialog({
        title: 'Task Group Deleted',
        message: 'This task group has been deleted!'
      }).onOk(() => {
        router.push({ name: 'home' })
      })
    } else if (payload.type === 'update' && _.has(payload, 'task')) {
      taskTable.value?.taskUpdated(payload.task)
      taskGraph.value?.taskUpdated(payload.task)
    } else if (payload.type === 'delete' && _.has(payload, 'task')) {
      taskTable.value?.taskDeleted(payload.task)
      taskGraph.value?.taskDeleted(payload.task)
    } else if (payload.type === 'create' && _.has(payload, 'task')) {
      taskTable.value?.taskCreated(payload.task)
      taskGraph.value?.taskCreated(payload.task)
    }
    throttledUpdateCompletedPercent()
  })
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
  watchGroup()
})

onBeforeUnmount(() => {
  unwatchGroup()
})
</script>
