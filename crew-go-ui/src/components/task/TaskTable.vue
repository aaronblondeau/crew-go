<template>
  <div class="full-width">
    <q-table
      :title="props.title"
      :rows="tasks"
      :columns="columns"
      v-model:pagination="paginationModel"
      row-key="id"
      :loading="loading"
      :flat="props.embedded"
      @request="onRequest"
      class="full-width"
      wrap-cells
    >
      <template v-slot:top>
        <div class="row full-width text-center" :class="{'text-left': !$q.screen.xs}">
          <div v-if="props.title" class="col-12 col-sm-3">
            <div class="q-table__title">
              {{ props.title }}
            </div>
          </div>

          <div :class="{
            'q-mt-sm': $q.screen.xs,
            'col-12 col-sm-5': props.title && !hideCreateButtons,
            'col-12 col-sm-9': props.title && hideCreateButtons,
            'col-12 col-sm-8': !props.title && !hideCreateButtons,
            'col-12': !props.title && props.hideCreateButtons
          }">
            <q-input
              type="text"
              v-model="search"
              placeholder="Search"
              filled
              dense
              @keyup.enter="loadTasks"
            >
              <template v-slot:after>
                <q-btn round dense flat icon="search" @click="loadTasks" />
                <q-btn round dense flat icon="clear" @click="clearSearch" />
                <q-btn round dense flat icon="refresh" @click="loadTasks" />
              </template>
            </q-input>
          </div>
          <div v-if="!props.hideCreateButtons" class="col-12 col-sm-4 text-center" :class="{'text-right': !$q.screen.xs, 'q-mt-sm': $q.screen.xs}">
            <CreateTaskModalButton :task-group="props.taskGroup" @on-create="onCreate" />
          </div>
        </div>
      </template>

      <template v-slot:body="props">
        <q-tr :props="props" valign="top">
          <q-td key="id" :props="props">
            <q-btn flat icon="content_copy" size="sm" @click="toClipboard('Id', props.row.id)" />
            <!-- {{ props.row.id }} -->
          </q-td>

          <q-td key="name" :props="props">
            {{ props.row.name }}
          </q-td>

          <q-td key="worker" :props="props">
            {{ props.row.worker }}
          </q-td>

          <q-td key="isPaused" :props="props">
            {{ props.row.isPaused ? 'Yes' : 'No' }}
          </q-td>

          <q-td key="isComplete" :props="props">
            <span v-if="props.row.busyExecuting">
              <q-chip size="md" color="purple" text-color="white" icon="pending">
                Executing
              </q-chip>
            </span>
            <span v-else>
              <q-chip v-if="props.row.isComplete" size="md" color="green" text-color="white" icon="done">
                Yes
              </q-chip>
              <q-chip v-if="!props.row.isComplete && (!props.row.errors ||  props.row.errors.length === 0)" size="md" color="blue" text-color="white" icon="hourglass_empty">
                No
              </q-chip>
              <q-chip v-if="!props.row.isComplete && props.row.errors && props.row.errors.length > 0" size="md" color="orange" text-color="white" icon="warning">
                No
              </q-chip>
            </span>
          </q-td>

          <q-td key="actions" :props="props">
            <CreateTaskModalButton label="" size="sm" flat :task-group="rootProps.taskGroup" :parent-id="props.row.id" @on-create="onCreate">
              <q-tooltip>
                Add a child to this task
              </q-tooltip>
            </CreateTaskModalButton>
            <EditTaskModalButton label="" size="sm" flat :task-group="rootProps.taskGroup" :task="props.row" @on-update="onUpdate" />
            <DeleteTaskModalButton label="" size="sm" flat :task="props.row" @on-delete="onDelete" />

            <ResetTaskModalButton :task="props.row" @on-reset="(evt: any) => onReset(evt, props.row)" />
            <RetryTaskModalButton :task="props.row" @on-retry="(evt: any) => onRetry(evt, props.row)" />

            <q-btn v-if="!props.row.isComplete && !props.row.isPaused" flat icon="pause" color="primary" size="sm" @click="pauseTask(props.row)">
              <q-tooltip>
                Pause this task
              </q-tooltip>
            </q-btn>
            <q-btn v-if="!props.row.isComplete && props.row.isPaused" flat icon="play_arrow" color="primary" size="sm" @click="resumeTask(props.row)">
              <q-tooltip>
                Resume this task
              </q-tooltip>
            </q-btn>

            <TaskOutputModalButton :task="props.row" />
          </q-td>
        </q-tr>
      </template>
    </q-table>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import toClipboard from 'src/lib/toClipboard'
import { useRouter, useRoute } from 'vue-router'
import _ from 'lodash'
import notifyError from 'src/lib/notifyError'
import CreateTaskModalButton from 'src/components/task/CreateTaskModalButton.vue'
import EditTaskModalButton from 'src/components/task/EditTaskModalButton.vue'
import DeleteTaskModalButton from 'src/components/task/DeleteTaskModalButton.vue'
import ResetTaskModalButton from 'src/components/task/ResetTaskModalButton.vue'
import RetryTaskModalButton from 'src/components/task/RetryTaskModalButton.vue'
import TaskOutputModalButton from 'src/components/task/TaskOutputModalButton.vue'
import { QTableProps } from 'quasar'
import { useTaskStore, Task } from 'src/stores/task-store'
import { TaskGroup } from 'src/stores/task-group-store'

const taskStore = useTaskStore()

const router = useRouter()
const route = useRoute()

interface Props {
  title?: string;
  taskGroup: TaskGroup;
  hideCreateButtons?: boolean;
  embedded?: boolean;
}
const props = withDefaults(defineProps<Props>(), { title: 'Tasks', hideCreateButtons: false, embedded: false })
const rootProps = props

const loading = ref(true)
const tasks = ref<Array<Task>>([])
const paginationModel = ref<QTableProps['pagination']>({
  page: parseInt(router.currentRoute.value.query.page ? router.currentRoute.value.query.page as string : '1'),
  rowsPerPage: router.currentRoute.value.query.page_size ? parseInt(router.currentRoute.value.query.page_size as string) : 20,
  rowsNumber: 0
})
const search = ref(router.currentRoute.value.query.q as string || '')

const columns : QTableProps['columns'] = [
  {
    name: 'id',
    field: 'id',
    label: 'Id',
    align: 'left'
  },
  {
    name: 'name',
    field: 'name',
    label: 'Name',
    align: 'left'
  },
  {
    name: 'worker',
    field: 'worker',
    label: 'Worker',
    align: 'left'
  },
  {
    name: 'isPaused',
    field: 'isPaused',
    label: 'Paused',
    align: 'left'
  },
  {
    name: 'isComplete',
    field: 'isComplete',
    label: 'Complete',
    align: 'left'
  },
  {
    name: 'actions',
    field: '',
    label: 'Actions',
    align: 'left'
  }
]

function onCreate (task: Task) {
  console.log('~~ task created', task)
  loadTasks()
}

function onUpdate (task: Task) {
  loadTasks()
}

function onDelete (taskId: string) {
  console.log('~~ task deleted', taskId)
  loadTasks()
}

async function clearSearch () {
  search.value = ''
  await loadTasks()
}

const onRequest : QTableProps['onRequest'] = async ({ pagination }) => {
  paginationModel.value = pagination
  await loadTasks()
}

async function loadTasks () {
  try {
    if (paginationModel.value) {
      loading.value = true
      const result = await taskStore.getTasks(props.taskGroup.id, paginationModel.value.page, paginationModel.value.rowsPerPage, search.value)
      paginationModel.value.rowsNumber = result.count
      for (const task of result.tasks) {
        task.pauseWait = false
        task.resumeWait = false
        task.resetWait = false
        task.retryWait = false
      }
      tasks.value = result.tasks
      router.push({ query: { ...route.query, q: search.value, page: paginationModel.value.page, page_size: paginationModel.value.rowsPerPage } })
    }
  } catch (e) {
    notifyError(e)
  } finally {
    loading.value = false
  }
}

async function pauseTask (task: Task) {
  try {
    task.pauseWait = true
    await taskStore.pauseTask(props.taskGroup.id, task.id)
    task.isPaused = true
  } catch (e) {
    notifyError(e)
  } finally {
    task.pauseWait = false
  }
}

async function resumeTask (task: Task) {
  try {
    task.pauseWait = true
    await taskStore.resumeTask(props.taskGroup.id, task.id)
    task.isPaused = false
  } catch (e) {
    notifyError(e)
  } finally {
    task.pauseWait = false
  }
}

function onReset (evt: any, task: Task) {
  Object.assign(task, evt)
}

function onRetry (evt: any, task: Task) {
  Object.assign(task, evt)
}

function taskUpdated (update: Task) {
  for (const task of tasks.value) {
    if (task.id === update.id) {
      Object.assign(task, update)
      break
    }
  }
}

function taskDeleted (deleted: Task) {
  for (const task of tasks.value) {
    if (task.id === deleted.id) {
      tasks.value = _.without(tasks.value, task)
      break
    }
  }
}

function taskCreated (task: Task) {
  let rpp = 0
  if (paginationModel.value && paginationModel.value.rowsPerPage) {
    rpp = paginationModel.value.rowsPerPage
  }
  if ((rpp === 0) || (tasks.value.length < rpp)) {
    tasks.value.push(task)
  }
  // ???
  // else {
  //   loadTasks()
  // }
}

defineExpose({
  loadTasks,
  taskUpdated,
  taskDeleted,
  taskCreated
})

onMounted(async () => {
  await loadTasks()
})
</script>
