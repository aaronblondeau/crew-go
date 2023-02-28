import { Notify } from 'quasar'
import _ from 'lodash'

export default function notifyError (error: unknown) {
  console.error(error)
  if (error instanceof Error) {
    Notify.create({
      type: 'negative',
      message: error.message + ''
    })
  } else {
    if (_.has(error, 'message')) {
      Notify.create({
        type: 'negative',
        message: (error as { message: string }).message + ''
      })
    } else {
      Notify.create({
        type: 'negative',
        message: error + ''
      })
    }
  }
}
