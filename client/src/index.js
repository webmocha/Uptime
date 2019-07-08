import { stringify } from "querystring"
import { run } from "@cycle/run"
import { withState } from '@cycle/state';
import { button, p, h1, h4, a, div, table, th, tr, td, fieldset, input, makeDOMDriver } from "@cycle/dom"
import { makeHTTPDriver } from "@cycle/http"
import xs from "xstream"

const sitesRequest = {
  url:'/api/sites',
  method: 'GET',
  category: 'sites',
}

const addSiteRequest = query => ({
  url:'/api/sites',
  method: 'POST',
  category: 'addSite',
  send: stringify(query)
})

function main(sources) {

  const formState = {
    key: ''
  }

  const addSiteInputChange$ = sources.DOM.select('.add-site-input').events('change')
    .map(ev => ev.target.value)
    .startWith('')
    .addListener({
      next: value => formState.key = value,
      error: console.error,
      complete: () => {},
    })

  const addSite$ = sources.DOM.select('.add-site-btn').events('click')
    .map(() => addSiteRequest({ key: formState.key }))

  const addSiteResponse$ = sources.HTTP
    .select('addSite')
    .flatten()
    .map(res => ({ addSiteStatus: res.text }));

  const initStatus$ = xs.of(sitesRequest)

  const periodicStatus$ = xs.periodic(60 * 1000)
    .mapTo(sitesRequest)

  const remove$ = sources.DOM.select('[data-action="remove"]')
    .events('click')
    .map(ev => ev.currentTarget.dataset['key']);

  const sitesResponse$ = sources.HTTP
    .select('sites')
    .flatten()
    .map(res => ({sites: res.body}));

  const initialReducer$ = xs.of(() => ({
    sites: [],
    addSiteStatus: ''
  }))

  const sitesReducer$ = sitesResponse$
    .map(({ sites }) => state => ({ ...state, sites }))

  const addSiteStatusReducer$ = addSiteResponse$
    .map(({ addSiteStatus }) => state => ({ ...state, addSiteStatus }))

  const reducer$ = xs.merge(initialReducer$, sitesReducer$, addSiteStatusReducer$);

  const state$ = sources.state.stream;

  const vdom$ = state$
    .map(({ sites, addSiteStatus }) =>
      div([
        fieldset([
          input('.add-site-input'),
          button('.add-site-btn', 'Add Site'),
          div(addSiteStatus),
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
    HTTP: xs.merge(initStatus$, periodicStatus$, addSite$),
    state: reducer$
  }
}

const drivers = {
  DOM: makeDOMDriver('#app'),
  HTTP: makeHTTPDriver(),
}

run(withState(main), drivers);
