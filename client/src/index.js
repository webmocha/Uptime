import { run } from "@cycle/run"
import { button, p, h1, h4, a, div, table, th, tr, td, fieldset, input, makeDOMDriver } from "@cycle/dom"
import { makeHTTPDriver } from "@cycle/http"
import xs from "xstream"

const initialState$ = xs.of({
  addKey: '',
  sites: [],
})

function main(sources) {
  const addSiteInputChange$ = sources.DOM.select('.add-site-input').events('change')
    .map(ev => ev.target.value)
    .startWith('')
    .map(addKey => { return console.log('KEY', addKey) || { addKey } })

  const addSiteClick$ = sources.DOM.select('.add-site-btn').events('click');


  const request = {
    url:'/api/sites',
    method: 'GET',
    category: 'sites',
  }

  const initStatus$ = xs.of(request)

  const periodicStatus$ = xs.periodic(60 * 1000)
    .mapTo(request)

  const remove$ = sources.DOM.select('[data-action="remove"]')
    .events('click')
    .map(ev => ev.currentTarget.dataset['key']);

  const response$ = sources.HTTP
    .select('sites')
    .flatten()
    .map(res => ({sites: res.body}));

  const state$ = initialState$
    .map(props => xs.combine(
      response$,
      addSiteInputChange$
    ))
    .flatten()
    .map(combined => combined.reduce((combined, part) => ({ ...part, ...combined }), {}))
    .remember()

  const vdom$ = state$
    .map(({ addKey, sites }) =>
      div([
        div(addKey),
        fieldset([
          input('.add-site-input'),
          button('.add-site-btn', 'Add Site'),
        ]),
        table([
          tr([
            th('Key'),
            th('Uptime'),
            th({ attrs : { colspan: 2}}, 'Status'),
            th('History'),
            th(),
          ]),
          ...sites.map(({key, uptime, status, statusText}) =>
            tr([
              td(key),
              td(uptime),
              td(status),
              td(statusText),
              td(
                a({
                  attrs: {
                    href: `/site?key=${key}`
                  }
                },
                button({ attrs: { type: 'button' }}, 'H'))
              ),
              td(
                button({ attrs: {
                  type: 'button',
                  'data-key': key,
                  'data-action': 'remove'
                }}, 'X')
              ),
            ])
          )
        ]),
      ])
    )

  return {
    DOM: vdom$,
    HTTP: xs.merge(initStatus$, periodicStatus$),
  }
}

const drivers = {
  DOM: makeDOMDriver('#app'),
  HTTP: makeHTTPDriver(),
}

run(main, drivers);
