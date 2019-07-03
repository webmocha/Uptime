import { run } from "@cycle/run"
import { button, p, h1, h4, a, div, table, th, tr, td, fieldset, input, makeDOMDriver } from "@cycle/dom"
import { makeHTTPDriver } from "@cycle/http"
import xs from "xstream"

function main(sources) {
    // const click$ = sources.DOM.select('.get-first').events('click');

    const request$ = xs.of({
        url:'/api/sites',
        method: 'GET',
        category: 'sites',
    })

    const stream = xs.periodic(5 * 1000)
      .map(() => xs.of({
          url:'/api/sites',
          method: 'GET',
          category: 'sites',
      }))

    const remove$ = sources.DOM.select('[data-action="remove"]')
      .events('click')
      .map(ev => ev.currentTarget.dataset['key']);

    const response$ = sources.HTTP
        .select('sites')
        .flatten()
        .map(res => res.body);

    const vdom$ = response$.startWith([]).map(sites =>
        div([
          fieldset([
            input('.add-site'),
            button('Add Site'),
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
      HTTP: request$,
    }
}

const drivers = {
    DOM: makeDOMDriver('#app'),
    HTTP: makeHTTPDriver(),
}

run(main, drivers);
