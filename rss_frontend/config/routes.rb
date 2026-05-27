Rails.application.routes.draw do
  devise_for :users, path: "api/v1/users", controllers: { sessions: "api/v1/sessions" }

  namespace :api do
    namespace :v1 do
      resources :feeds, only: [ :create ]
      resources :feed_items, only: [ :index ]
      get "health", to: "health#index"
    end
  end

  # React SPA catch-all routing
  root to: "home#index"
  get "*path", to: "home#index", constraints: ->(req) { !req.xhr? && req.format.html? }
end
