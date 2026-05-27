module FeatureHelpers
  def sign_in_as(user)
    visit "/login"
    fill_in "Email", with: user.email
    fill_in "Password", with: "password"
    click_button "Sign in"
    expect(page).to have_current_path("/feeds")
  end
end

RSpec.configure do |config|
  config.include Devise::Test::IntegrationHelpers, type: :request
  config.include Devise::Test::IntegrationHelpers, type: :feature
  config.include FeatureHelpers, type: :feature
end
