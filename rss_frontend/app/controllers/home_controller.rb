class HomeController < ApplicationController
  # Render the application layout shell that mounts the React SPA.
  def index
    render layout: "application", html: ""
  end
end
